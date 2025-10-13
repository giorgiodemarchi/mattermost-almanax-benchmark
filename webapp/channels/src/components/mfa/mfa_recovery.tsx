// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useState, useCallback } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';

import { Client4 } from 'mattermost-redux/client';

import FormButton from 'components/form_button';
import Input from 'components/widgets/inputs/input/input';
import LoadingScreen from 'components/loading_screen';

import './mfa_recovery.scss';

type Props = {
  onSuccess?: () => void;
  onCancel?: () => void;
};

type RecoveryMethod = 'email' | 'backup_code' | 'admin';

const MfaRecovery: React.FC<Props> = ({ onSuccess, onCancel }) => {
  const { formatMessage } = useIntl();
  const [step, setStep] = useState<'method' | 'email' | 'verify' | 'complete'>('method');
  const [recoveryMethod, setRecoveryMethod] = useState<RecoveryMethod>('email');
  const [email, setEmail] = useState('');
  const [recoveryCode, setRecoveryCode] = useState('');
  const [backupCode, setBackupCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const handleMethodSelect = useCallback((method: RecoveryMethod) => {
    setRecoveryMethod(method);
    if (method === 'email') {
      setStep('email');
    } else if (method === 'backup_code') {
      setStep('verify');
    }
  }, []);

  const handleRequestRecovery = useCallback(async () => {
    if (!email) {
      setError(formatMessage({ id: 'mfa.recovery.email_required', defaultMessage: 'Please enter your email address' }));
      return;
    }

    setLoading(true);
    setError('');

    try {
      const response = await Client4.doFetch<{ success: boolean; message: string }>(
        '/api/v4/users/mfa/recovery/request',
        {
          method: 'POST',
          body: JSON.stringify({
            email,
            recovery_method: recoveryMethod,
          }),
        },
      );

      if (response.success) {
        setSuccess(formatMessage({
          id: 'mfa.recovery.email_sent',
          defaultMessage: 'Recovery email sent. Please check your inbox.',
        }));
        setStep('verify');
      }
    } catch (err: any) {
      setError(err.message || formatMessage({
        id: 'mfa.recovery.request_failed',
        defaultMessage: 'Failed to request recovery. Please try again.',
      }));
    } finally {
      setLoading(false);
    }
  }, [email, recoveryMethod, formatMessage]);

  const handleVerifyCode = useCallback(async () => {
    if (!recoveryCode && !backupCode) {
      setError(formatMessage({ id: 'mfa.recovery.code_required', defaultMessage: 'Please enter the recovery code' }));
      return;
    }

    setLoading(true);
    setError('');

    try {
      const codeToVerify = recoveryMethod === 'backup_code' ? backupCode : recoveryCode;

      const response = await Client4.doFetch<{ success: boolean; message: string }>(
        '/api/v4/users/mfa/recovery/verify',
        {
          method: 'POST',
          body: JSON.stringify({
            email,
            recovery_code: codeToVerify,
          }),
        },
      );

      if (response.success) {
        setSuccess(formatMessage({
          id: 'mfa.recovery.code_verified',
          defaultMessage: 'Code verified successfully.',
        }));
        setStep('complete');
      }
    } catch (err: any) {
      setError(err.message || formatMessage({
        id: 'mfa.recovery.verify_failed',
        defaultMessage: 'Invalid or expired code. Please try again.',
      }));
    } finally {
      setLoading(false);
    }
  }, [recoveryCode, backupCode, email, recoveryMethod, formatMessage]);

  const handleCompleteRecovery = useCallback(async (resetMfa: boolean) => {
    setLoading(true);
    setError('');

    try {
      const codeToUse = recoveryMethod === 'backup_code' ? backupCode : recoveryCode;

      const response = await Client4.doFetch<{ success: boolean; message: string }>(
        '/api/v4/users/mfa/recovery/complete',
        {
          method: 'POST',
          body: JSON.stringify({
            email,
            recovery_code: codeToUse,
            reset_mfa: resetMfa,
          }),
        },
      );

      if (response.success) {
        setSuccess(formatMessage({
          id: 'mfa.recovery.completed',
          defaultMessage: 'MFA recovery completed successfully.',
        }));

        // Call onSuccess callback if provided
        if (onSuccess) {
          setTimeout(onSuccess, 2000);
        }
      }
    } catch (err: any) {
      setError(err.message || formatMessage({
        id: 'mfa.recovery.complete_failed',
        defaultMessage: 'Failed to complete recovery. Please try again.',
      }));
    } finally {
      setLoading(false);
    }
  }, [email, recoveryCode, backupCode, recoveryMethod, onSuccess, formatMessage]);

  if (loading) {
    return <LoadingScreen />;
  }

  const renderMethodSelection = () => (
    <div className='mfa-recovery-method-selection'>
      <h3>
        <FormattedMessage
          id='mfa.recovery.select_method'
          defaultMessage='Select Recovery Method'
        />
      </h3>
      <p>
        <FormattedMessage
          id='mfa.recovery.select_method_description'
          defaultMessage='Choose how you would like to recover access to your account:'
        />
      </p>
      <div className='recovery-method-buttons'>
        <button
          className='btn btn-primary recovery-method-btn'
          onClick={() => handleMethodSelect('email')}
        >
          <i className='icon icon-email-outline' />
          <FormattedMessage
            id='mfa.recovery.method.email'
            defaultMessage='Email Recovery'
          />
        </button>
        <button
          className='btn btn-secondary recovery-method-btn'
          onClick={() => handleMethodSelect('backup_code')}
        >
          <i className='icon icon-key-variant' />
          <FormattedMessage
            id='mfa.recovery.method.backup_code'
            defaultMessage='Use Backup Code'
          />
        </button>
      </div>
      {onCancel && (
        <button
          className='btn btn-tertiary cancel-btn'
          onClick={onCancel}
        >
          <FormattedMessage
            id='mfa.recovery.cancel'
            defaultMessage='Cancel'
          />
        </button>
      )}
    </div>
  );

  const renderEmailStep = () => (
    <div className='mfa-recovery-email-step'>
      <h3>
        <FormattedMessage
          id='mfa.recovery.email_step_title'
          defaultMessage='Enter Your Email'
        />
      </h3>
      <p>
        <FormattedMessage
          id='mfa.recovery.email_step_description'
          defaultMessage='Enter your email address to receive a recovery code.'
        />
      </p>
      <Input
        type='email'
        name='email'
        value={email}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEmail(e.target.value)}
        placeholder={formatMessage({
          id: 'mfa.recovery.email_placeholder',
          defaultMessage: 'Enter your email',
        })}
        disabled={loading}
      />
      <FormButton
        btnClass='btn-primary'
        saving={loading}
        onClick={handleRequestRecovery}
        defaultMessage={formatMessage({
          id: 'mfa.recovery.send_code',
          defaultMessage: 'Send Recovery Code',
        })}
        savingMessage={formatMessage({
          id: 'mfa.recovery.sending',
          defaultMessage: 'Sending...',
        })}
      />
      <button
        className='btn btn-tertiary back-btn'
        onClick={() => setStep('method')}
      >
        <FormattedMessage
          id='mfa.recovery.back'
          defaultMessage='Back'
        />
      </button>
    </div>
  );

  const renderVerifyStep = () => (
    <div className='mfa-recovery-verify-step'>
      <h3>
        <FormattedMessage
          id='mfa.recovery.verify_step_title'
          defaultMessage='Enter Recovery Code'
        />
      </h3>
      <p>
        {recoveryMethod === 'backup_code' ? (
          <FormattedMessage
            id='mfa.recovery.backup_code_description'
            defaultMessage='Enter one of your backup codes to verify your identity.'
          />
        ) : (
          <FormattedMessage
            id='mfa.recovery.verify_step_description'
            defaultMessage='Enter the recovery code sent to your email.'
          />
        )}
      </p>
      <Input
        type='text'
        name={recoveryMethod === 'backup_code' ? 'backup_code' : 'recovery_code'}
        value={recoveryMethod === 'backup_code' ? backupCode : recoveryCode}
        onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
          if (recoveryMethod === 'backup_code') {
            setBackupCode(e.target.value);
          } else {
            setRecoveryCode(e.target.value);
          }
        }}
        placeholder={formatMessage({
          id: recoveryMethod === 'backup_code' ? 'mfa.recovery.backup_code_placeholder' : 'mfa.recovery.code_placeholder',
          defaultMessage: 'Enter code',
        })}
        disabled={loading}
      />
      <FormButton
        btnClass='btn-primary'
        saving={loading}
        onClick={handleVerifyCode}
        defaultMessage={formatMessage({
          id: 'mfa.recovery.verify',
          defaultMessage: 'Verify Code',
        })}
        savingMessage={formatMessage({
          id: 'mfa.recovery.verifying',
          defaultMessage: 'Verifying...',
        })}
      />
      <button
        className='btn btn-tertiary back-btn'
        onClick={() => setStep(recoveryMethod === 'backup_code' ? 'method' : 'email')}
      >
        <FormattedMessage
          id='mfa.recovery.back'
          defaultMessage='Back'
        />
      </button>
    </div>
  );

  const renderCompleteStep = () => (
    <div className='mfa-recovery-complete-step'>
      <h3>
        <FormattedMessage
          id='mfa.recovery.complete_step_title'
          defaultMessage='Complete Recovery'
        />
      </h3>
      <p>
        <FormattedMessage
          id='mfa.recovery.complete_step_description'
          defaultMessage='Choose what you would like to do:'
        />
      </p>
      <div className='complete-options'>
        <FormButton
          btnClass='btn-primary'
          saving={loading}
          onClick={() => handleCompleteRecovery(true)}
          defaultMessage={formatMessage({
            id: 'mfa.recovery.reset_mfa',
            defaultMessage: 'Reset MFA and Login',
          })}
          savingMessage={formatMessage({
            id: 'mfa.recovery.resetting',
            defaultMessage: 'Resetting...',
          })}
        />
        <FormButton
          btnClass='btn-secondary'
          saving={loading}
          onClick={() => handleCompleteRecovery(false)}
          defaultMessage={formatMessage({
            id: 'mfa.recovery.reconfigure_mfa',
            defaultMessage: 'Reconfigure MFA',
          })}
          savingMessage={formatMessage({
            id: 'mfa.recovery.saving',
            defaultMessage: 'Saving...',
          })}
        />
      </div>
    </div>
  );

  return (
    <div className='mfa-recovery-container'>
      <div className='mfa-recovery-content'>
        {error && (
          <div className='alert alert-danger'>
            {error}
          </div>
        )}
        {success && (
          <div className='alert alert-success'>
            {success}
          </div>
        )}
        {step === 'method' && renderMethodSelection()}
        {step === 'email' && renderEmailStep()}
        {step === 'verify' && renderVerifyStep()}
        {step === 'complete' && renderCompleteStep()}
      </div>
    </div>
  );
};

export default MfaRecovery;

