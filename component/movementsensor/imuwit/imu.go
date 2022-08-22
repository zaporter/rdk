// Package imuwit implements a wit IMU.
package imuwit

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

const model = "imu_wit"

func init() {
	registry.RegisterComponent(movementsensor.Subtype, model, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewWit(deps, config, logger)
		},
	})
}

type wit struct {
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles
	acceleration    r3.Vector
	magnetometer    r3.Vector
	lastError       error

	mu sync.Mutex

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	generic.Unimplemented
}

func (imu *wit) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.angularVelocity, imu.lastError
}

func (imu *wit) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return r3.Vector{}, imu.lastError
}

func (imu *wit) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return &imu.orientation, imu.lastError
}

// GetAcceleration returns accelerometer acceleration in mm_per_sec_per_sec.
func (imu *wit) GetAcceleration(ctx context.Context) (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.acceleration, imu.lastError
}

// GetMagnetometer returns magnetic field in gauss.
func (imu *wit) GetMagnetometer(ctx context.Context) (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.magnetometer, imu.lastError
}

func (imu *wit) GetCompassHeading(ctx context.Context) (float64, error) {
	// TODO(erh): is this right? I don't think so
	return imu.magnetometer.Z, imu.lastError
}

func (imu *wit) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	return nil, 0, nil
}

func (imu *wit) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	return map[string]float32{}, nil
}

func (imu *wit) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	return movementsensor.GetReadings(ctx, imu)
}

func (imu *wit) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		AngularVelocitySupported: true,
		OrientationSupported:     true,
		CompassHeadingSupported:  true,
	}, nil
}

// NewWit creates a new Wit IMU.
func NewWit(deps registry.Dependencies, config config.Component, logger golog.Logger) (movementsensor.MovementSensor, error) {
	options := slib.OpenOptions{
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	options.PortName = config.Attributes.String("port")
	if options.PortName == "" {
		return nil, errors.New("wit imu needs a port")
	}

	port, err := slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(port)

	var i wit

	var ctx context.Context
	ctx, i.cancelFunc = context.WithCancel(context.Background())
	i.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer utils.UncheckedErrorFunc(port.Close)
		defer i.activeBackgroundWorkers.Done()

		for {
			if ctx.Err() != nil {
				return
			}

			line, err := portReader.ReadString('U')

			func() {
				i.mu.Lock()
				defer i.mu.Unlock()

				if err != nil {
					i.lastError = err
				} else {
					i.lastError = i.parseWIT(line)
				}
			}()
		}
	})
	return &i, nil
}

func scale(a, b byte, r float64) float64 {
	x := float64(int(b)<<8|int(a)) / 32768.0 // 0 -> 2
	x *= r                                   // 0 -> 2r
	x += r
	x = math.Mod(x, r*2)
	x -= r
	return x
}

func scalemag(a, b byte, r float64) float64 {
	x := float64(int(b)<<8 | int(a)) // 0 -> 2
	x *= r                           // 0 -> 2r
	x += r
	x = math.Mod(x, r*2)
	x -= r
	return x
}

func (imu *wit) parseWIT(line string) error {
	if line[0] == 0x52 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu angularVelocity %d %v", len(line), line)
		}
		imu.angularVelocity.X = scale(line[1], line[2], 2000)
		imu.angularVelocity.Y = scale(line[3], line[4], 2000)
		imu.angularVelocity.Z = scale(line[5], line[6], 2000)
	}

	if line[0] == 0x53 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu orientation %d %v", len(line), line)
		}

		imu.orientation.Roll = rutils.DegToRad(scale(line[1], line[2], 180))
		imu.orientation.Pitch = rutils.DegToRad(scale(line[3], line[4], 180))
		imu.orientation.Yaw = rutils.DegToRad(scale(line[5], line[6], 180))
	}

	if line[0] == 0x51 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu acceleration %d %v", len(line), line)
		}
		imu.acceleration.X = scale(line[1], line[2], 16) * 9806.65 // converts of mm_per_sec_per_sec in NYC
		imu.acceleration.Y = scale(line[3], line[4], 16) * 9806.65
		imu.acceleration.Z = scale(line[5], line[6], 16) * 9806.65
	}

	if line[0] == 0x54 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu magnetometer %d %v", len(line), line)
		}
		imu.magnetometer.X = scalemag(line[1], line[2], 1) * 0.01 // converts to gauss
		imu.magnetometer.Y = scalemag(line[3], line[4], 1) * 0.01
		imu.magnetometer.Z = scalemag(line[5], line[6], 1) * 0.01
	}

	return nil
}

func (imu *wit) Close() {
	imu.cancelFunc()
	imu.activeBackgroundWorkers.Wait()
}