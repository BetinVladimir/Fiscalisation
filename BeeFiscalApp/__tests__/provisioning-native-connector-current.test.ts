let mockScan:jest.Mock,mockStop:jest.Mock,mockCancel:jest.Mock,mockDevice:any;
jest.mock('react-native',()=>({Platform:{OS:'ios',Version:18},PermissionsAndroid:{}}));
jest.mock('react-native-ble-plx',()=>({BleManager:jest.fn().mockImplementation(()=>({startDeviceScan:(...args:any[])=>mockScan(...args),stopDeviceScan:()=>mockStop(),cancelDeviceConnection:(...args:any[])=>mockCancel(...args)}))}));
import {connectProvisioningDevice} from '../src/provisioningBle.native';
beforeEach(()=>{
 mockStop=jest.fn();mockCancel=jest.fn().mockResolvedValue(undefined);
 mockDevice={id:'synthetic-s3',mtu:185,connect:jest.fn(),discoverAllServicesAndCharacteristics:jest.fn().mockResolvedValue(undefined)};
 mockDevice.connect.mockResolvedValue(mockDevice);
 mockScan=jest.fn((_services,_options,callback)=>callback(null,mockDevice));
});
test('failed actual native discovery closes the connected device and preserves the error',async()=>{
 const failure=Error('synthetic discovery failure');mockDevice.discoverAllServicesAndCharacteristics.mockRejectedValue(failure);
 await expect(connectProvisioningDevice()).rejects.toBe(failure);
 expect(mockStop).toHaveBeenCalledTimes(1);expect(mockCancel).toHaveBeenCalledWith(mockDevice.id);
});
test('small MTU closes the connected device before rejecting setup',async()=>{
 mockDevice.mtu=61;await expect(connectProvisioningDevice()).rejects.toThrow('PROVISIONING_ATT_MTU_TOO_SMALL');
 expect(mockCancel).toHaveBeenCalledWith(mockDevice.id);
});
test('successful setup leaves device connected until owner disconnects',async()=>{
 const link=await connectProvisioningDevice();expect(mockCancel).not.toHaveBeenCalled();await link.disconnect();expect(mockCancel).toHaveBeenCalledWith(mockDevice.id);
});
