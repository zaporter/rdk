import { Client, commonApi, navigationApi } from '@viamrobotics/sdk';
import { grpc } from '@improbable-eng/grpc-web';
import { rcLogConditionally } from '@/lib/log';

export type NavigationModes =
  | typeof navigationApi.Mode.MODE_MANUAL
  | typeof navigationApi.Mode.MODE_UNSPECIFIED
  | typeof navigationApi.Mode.MODE_WAYPOINT

export const setMode = (client: Client, name: string, mode: NavigationModes) => {
  const request = new navigationApi.SetModeRequest();
  request.setName(name);
  request.setMode(mode);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.navigationService.setMode(request, new grpc.Metadata(), (error) => (
      error ? reject(error) : resolve(null)
    ));
  });
};

export const setWaypoint = (client: Client, lat: number, lng: number, name: string) => {
  const request = new navigationApi.AddWaypointRequest();
  const point = new commonApi.GeoPoint();

  point.setLatitude(lat);
  point.setLongitude(lng);
  request.setName(name);
  request.setLocation(point);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.navigationService.addWaypoint(request, new grpc.Metadata(), (error, response) =>
      (error ? reject(error) : resolve(response)));
  });
};

export const getWaypoints = (client: Client, name: string) => {
  const req = new navigationApi.GetWaypointsRequest();
  req.setName(name);

  rcLogConditionally(req);

  return new Promise<navigationApi.Waypoint[]>((resolve, reject) => {
    client.navigationService.getWaypoints(req, new grpc.Metadata(), (error, response) =>
      (error ? reject(error) : resolve(response?.getWaypointsList() ?? [])));
  });
};

export const removeWaypoint = (client: Client, name: string, id: string) => {
  const request = new navigationApi.RemoveWaypointRequest();
  request.setName(name);
  request.setId(id);

  rcLogConditionally(request);

  return new Promise((resolve, reject) => {
    client.navigationService.removeWaypoint(request, new grpc.Metadata(), (error) =>
      (error ? reject(error) : resolve(null)));
  });
};

export const getLocation = (client: Client, name: string) => {
  const request = new navigationApi.GetLocationRequest();
  request.setName(name);

  rcLogConditionally(request);

  return new Promise<{ lat: number, lng: number }>((resolve, reject) => {
    client.navigationService.getLocation(request, new grpc.Metadata(), (error, response) => (
      error
        ? reject(error)
        : resolve({
          lat: response?.getLocation()?.getLatitude() ?? 0,
          lng: response?.getLocation()?.getLongitude() ?? 0,
        })
    ));
  });
};
