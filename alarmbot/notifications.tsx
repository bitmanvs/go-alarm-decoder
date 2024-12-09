import {Notifications} from 'expo';
import * as Permissions from 'expo-permissions';
import Constants from 'expo-constants';
import {HOMES} from './config';
import {retry, post} from './networking';

const PUSH_ENDPOINT = '/register';

export async function registerForPushNotificationsAsync(): Promise<void> {
  Notifications.createChannelAndroidAsync('event', {
    name: 'Event',
    priority: 'high',
    description: 'General alarm events such as openning a door or turning the alarm off.',
    sound: true,
  });
  Notifications.createChannelAndroidAsync('alarm', {
    name: 'Alarming',
    description: 'Notifications when the alarm is sounding or a fire is detected.',
    priority: 'max',
    sound: true,
    vibrate: [0, 250, 250, 250],
  });

  const { status: existingStatus } = await Permissions.getAsync(
    Permissions.NOTIFICATIONS
  );
  let finalStatus = existingStatus;

  if (existingStatus !== 'granted') {
    const { status } = await Permissions.askAsync(Permissions.NOTIFICATIONS);
    finalStatus = status;
  }

  if (finalStatus !== 'granted') {
    return;
  }

  let token = await Notifications.getExpoPushTokenAsync();

  for (const home of HOMES) {
    retry(async (i: number) => {
      const endpoint = home.endpoints[i % home.endpoints.length] + PUSH_ENDPOINT;
      await post(endpoint, {
        Token: token,
        InstallationID: Constants.installationId,
        DeviceName: Constants.deviceId,
        NativeAppVersion: Constants.nativeAppVersion,
      });
    })
  }
}
