// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState, useEffect, useCallback} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import LoadingScreen from 'components/loading_screen';
import Input from 'components/widgets/inputs/input/input';

import './desktop_auth.scss';

interface DesktopLoginRequest {
    device_name: string;
    device_platform: string;
    app_version: string;
    login_method: string;
}

interface DesktopLoginResponse {
    session_token: string;
    device_code: string;
    expires_at: number;
    verify_url: string;
    qr_code?: string;
}

interface Props {
    location: {
        search: string;
    };
    history: {
        push: (path: string) => void;
    };
}

const DesktopAuth: React.FC<Props> = ({location, history}) => {
    const intl = useIntl();
    const [loading, setLoading] = useState(false);
    const [deviceCode, setDeviceCode] = useState('');
    const [sessionToken, setSessionToken] = useState('');
    const [expiresAt, setExpiresAt] = useState(0);
    const [error, setError] = useState('');
    const [verifyMode, setVerifyMode] = useState(false);
    const [deviceName, setDeviceName] = useState('');
    const [qrCode, setQRCode] = useState('');
    const [showQRCode, setShowQRCode] = useState(false);

    // Parse query parameters to check if we're verifying a code
    useEffect(() => {
        const params = new URLSearchParams(location.search);
        const code = params.get('code');
        const token = params.get('token');
        
        if (code) {
            setDeviceCode(code);
            setVerifyMode(true);
            if (token) {
                setSessionToken(token);
            }
        }
    }, [location.search]);

    // Initialize desktop login session
    const initializeDesktopLogin = useCallback(async () => {
        setLoading(true);
        setError('');

        try {
            const request: DesktopLoginRequest = {
                device_name: deviceName || 'Desktop App',
                device_platform: 'desktop',
                app_version: '5.0.0',
                login_method: showQRCode ? 'qr_code' : 'device_code',
            };

            const response: DesktopLoginResponse = await Client4.doFetch(
                `${Client4.getUsersRoute()}/login/desktop/init`,
                {
                    method: 'POST',
                    body: JSON.stringify(request),
                }
            );

            setDeviceCode(response.device_code);
            setSessionToken(response.session_token);
            setExpiresAt(response.expires_at);
            if (response.qr_code) {
                setQRCode(response.qr_code);
            }
        } catch (err: any) {
            setError(err.message || 'Failed to initialize desktop login');
        } finally {
            setLoading(false);
        }
    }, [deviceName, showQRCode]);

    // Verify device code
    const verifyDeviceCode = useCallback(async (code: string) => {
        setLoading(true);
        setError('');

        try {
            const session = await Client4.doFetch(
                `${Client4.getUsersRoute()}/login/desktop/verify?code=${code}`,
                {method: 'GET'}
            );

            return session;
        } catch (err: any) {
            setError(err.message || 'Failed to verify device code');
            return null;
        } finally {
            setLoading(false);
        }
    }, []);

    // Complete desktop login
    const completeDesktopLogin = useCallback(async (code: string) => {
        setLoading(true);
        setError('');

        try {
            await Client4.doFetch(
                `${Client4.getUsersRoute()}/login/desktop/complete`,
                {
                    method: 'POST',
                    body: JSON.stringify({device_code: code}),
                }
            );

            // Redirect to home after successful login
            history.push('/');
        } catch (err: any) {
            setError(err.message || 'Failed to complete desktop login');
        } finally {
            setLoading(false);
        }
    }, [history]);

    // Cancel desktop login
    const cancelDesktopLogin = useCallback(async (code: string) => {
        try {
            await Client4.doFetch(
                `${Client4.getUsersRoute()}/login/desktop/cancel`,
                {
                    method: 'POST',
                    body: JSON.stringify({device_code: code}),
                }
            );
        } catch (err: any) {
            console.error('Failed to cancel desktop login:', err);
        }
    }, []);

    // Handle device code input change
    const handleDeviceCodeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const value = e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '');
        setDeviceCode(value);
    };

    // Handle verify button click
    const handleVerify = async () => {
        if (!deviceCode || deviceCode.length !== 8) {
            setError('Please enter a valid 8-character device code');
            return;
        }

        const session = await verifyDeviceCode(deviceCode);
        if (session) {
            await completeDesktopLogin(deviceCode);
        }
    };

    // Handle cancel
    const handleCancel = async () => {
        if (deviceCode) {
            await cancelDesktopLogin(deviceCode);
        }
        history.push('/login');
    };

    // Auto-refresh expiry countdown
    useEffect(() => {
        if (!expiresAt) {
            return undefined;
        }

        const interval = setInterval(() => {
            const now = Date.now();
            if (now >= expiresAt) {
                setError('Device code has expired. Please generate a new one.');
                clearInterval(interval);
            }
        }, 1000);

        return () => clearInterval(interval);
    }, [expiresAt]);

    // Calculate remaining time
    const getRemainingTime = () => {
        if (!expiresAt) {
            return '';
        }

        const remaining = Math.max(0, expiresAt - Date.now());
        const minutes = Math.floor(remaining / 60000);
        const seconds = Math.floor((remaining % 60000) / 1000);
        return `${minutes}:${seconds.toString().padStart(2, '0')}`;
    };

    if (loading) {
        return <LoadingScreen/>;
    }

    // Verification mode (scanning QR or entering code)
    if (verifyMode) {
        return (
            <div className='desktop-auth-container'>
                <div className='desktop-auth-card'>
                    <div className='desktop-auth-header'>
                        <h1>
                            <FormattedMessage
                                id='desktop_auth.verify.title'
                                defaultMessage='Verify Desktop Login'
                            />
                        </h1>
                        <p>
                            <FormattedMessage
                                id='desktop_auth.verify.description'
                                defaultMessage='Confirm this login request to continue'
                            />
                        </p>
                    </div>

                    {sessionToken && (
                        <div className='desktop-auth-session-info'>
                            <p className='device-code-display'>
                                <FormattedMessage
                                    id='desktop_auth.device_code'
                                    defaultMessage='Device Code: {code}'
                                    values={{code: deviceCode}}
                                />
                            </p>
                        </div>
                    )}

                    {error && (
                        <div className='desktop-auth-error'>
                            {error}
                        </div>
                    )}

                    <div className='desktop-auth-actions'>
                        <button
                            className='btn btn-primary'
                            onClick={handleVerify}
                            disabled={loading}
                        >
                            <FormattedMessage
                                id='desktop_auth.verify.button'
                                defaultMessage='Confirm Login'
                            />
                        </button>
                        <button
                            className='btn btn-secondary'
                            onClick={handleCancel}
                            disabled={loading}
                        >
                            <FormattedMessage
                                id='desktop_auth.cancel.button'
                                defaultMessage='Cancel'
                            />
                        </button>
                    </div>
                </div>
            </div>
        );
    }

    // Initiation mode (desktop app requesting login)
    return (
        <div className='desktop-auth-container'>
            <div className='desktop-auth-card'>
                <div className='desktop-auth-header'>
                    <h1>
                        <FormattedMessage
                            id='desktop_auth.title'
                            defaultMessage='Desktop App Login'
                        />
                    </h1>
                    <p>
                        <FormattedMessage
                            id='desktop_auth.description'
                            defaultMessage='Login to Mattermost desktop app by scanning the QR code or entering the device code'
                        />
                    </p>
                </div>

                {!deviceCode && (
                    <>
                        <div className='desktop-auth-input-group'>
                            <Input
                                type='text'
                                name='device_name'
                                value={deviceName}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setDeviceName(e.target.value)}
                                placeholder={intl.formatMessage({
                                    id: 'desktop_auth.device_name.placeholder',
                                    defaultMessage: 'Device Name (e.g., Work Laptop)',
                                })}
                            />
                        </div>

                        <div className='desktop-auth-login-method'>
                            <label className='desktop-auth-checkbox'>
                                <input
                                    type='checkbox'
                                    checked={showQRCode}
                                    onChange={(e) => setShowQRCode(e.target.checked)}
                                />
                                <FormattedMessage
                                    id='desktop_auth.use_qr_code'
                                    defaultMessage='Use QR Code'
                                />
                            </label>
                        </div>

                        <button
                            className='btn btn-primary'
                            onClick={initializeDesktopLogin}
                            disabled={loading}
                        >
                            <FormattedMessage
                                id='desktop_auth.generate.button'
                                defaultMessage='Generate Login Code'
                            />
                        </button>
                    </>
                )}

                {deviceCode && !showQRCode && (
                    <div className='desktop-auth-code-display'>
                        <h2>
                            <FormattedMessage
                                id='desktop_auth.your_code'
                                defaultMessage='Your Device Code'
                            />
                        </h2>
                        <div className='device-code-large'>
                            {deviceCode}
                        </div>
                        <p className='device-code-instruction'>
                            <FormattedMessage
                                id='desktop_auth.code_instruction'
                                defaultMessage='Enter this code in your desktop app'
                            />
                        </p>
                        <p className='device-code-expiry'>
                            <FormattedMessage
                                id='desktop_auth.expires_in'
                                defaultMessage='Expires in: {time}'
                                values={{time: getRemainingTime()}}
                            />
                        </p>
                    </div>
                )}

                {deviceCode && showQRCode && qrCode && (
                    <div className='desktop-auth-qr-display'>
                        <h2>
                            <FormattedMessage
                                id='desktop_auth.scan_qr'
                                defaultMessage='Scan QR Code'
                            />
                        </h2>
                        <div className='qr-code-container'>
                            <img
                                src={`data:image/svg+xml;base64,${btoa(generateQRCodeSVG(qrCode))}`}
                                alt='QR Code for desktop login'
                            />
                        </div>
                        <p className='qr-code-instruction'>
                            <FormattedMessage
                                id='desktop_auth.qr_instruction'
                                defaultMessage='Scan this QR code with your mobile device to login'
                            />
                        </p>
                        <p className='device-code-expiry'>
                            <FormattedMessage
                                id='desktop_auth.expires_in'
                                defaultMessage='Expires in: {time}'
                                values={{time: getRemainingTime()}}
                            />
                        </p>
                    </div>
                )}

                {error && (
                    <div className='desktop-auth-error'>
                        {error}
                    </div>
                )}

                {deviceCode && (
                    <div className='desktop-auth-manual-entry'>
                        <h3>
                            <FormattedMessage
                                id='desktop_auth.or_enter_manually'
                                defaultMessage='Or enter code manually'
                            />
                        </h3>
                        <div className='desktop-auth-input-group'>
                            <Input
                                type='text'
                                name='manual_code'
                                value={deviceCode}
                                onChange={handleDeviceCodeChange}
                                placeholder='Enter 8-character code'
                                maxLength={8}
                            />
                        </div>
                        <button
                            className='btn btn-secondary'
                            onClick={handleCancel}
                        >
                            <FormattedMessage
                                id='desktop_auth.cancel.button'
                                defaultMessage='Cancel'
                            />
                        </button>
                    </div>
                )}
            </div>
        </div>
    );
};

// Simple QR code SVG generator (for demonstration)
// In production, you'd use a proper QR code library
function generateQRCodeSVG(data: string): string {
    const size = 200;
    const modules = 25; // QR code modules
    const moduleSize = size / modules;

    // This is a simplified placeholder
    // In a real implementation, use a QR code generation library
    return `
        <svg width="${size}" height="${size}" xmlns="http://www.w3.org/2000/svg">
            <rect width="${size}" height="${size}" fill="white"/>
            <text x="50%" y="50%" text-anchor="middle" dy=".3em" font-size="12">
                QR Code: ${data}
            </text>
        </svg>
    `;
}

export default DesktopAuth;

