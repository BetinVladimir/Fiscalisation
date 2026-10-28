import Constants from 'expo-constants';
export const demoEmailOtpEnabled = () => Constants.expoConfig?.extra?.isDemo === true && Constants.expoConfig?.extra?.appVariant === 'demo';
export const validEmailOtp = (value: string, demo: boolean) => /^\d{6}$/.test(value) || (demo === true && value === 'demo');
export const normalizeEmailOtp = (value: string, demo: boolean) => (demo === true ? value : value.replace(/\D/g, '')).slice(0, 6);
