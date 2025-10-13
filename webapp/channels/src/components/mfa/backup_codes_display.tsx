// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, { useState, useEffect, useCallback } from 'react';
import { FormattedMessage, useIntl } from 'react-intl';

import { Client4 } from 'mattermost-redux/client';

import LoadingScreen from 'components/loading_screen';

import './backup_codes_display.scss';

type Props = {
  userId: string;
};

interface BackupCodesResponse {
  backup_codes: string[];
  generated_at: number;
}

const BackupCodesDisplay: React.FC<Props> = ({ userId }) => {
  const { formatMessage } = useIntl();
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState(false);
  const [hasGenerated, setHasGenerated] = useState(false);

  const generateCodes = useCallback(async () => {
    setLoading(true);
    setError('');

    try {
      const response = await Client4.doFetch<BackupCodesResponse>(
        `/api/v4/users/${userId}/mfa/backup_codes`,
        { method: 'POST' },
      );

      setBackupCodes(response.backup_codes);
      setHasGenerated(true);
    } catch (err: any) {
      setError(err.message || formatMessage({
        id: 'mfa.backup_codes.generate_error',
        defaultMessage: 'Failed to generate backup codes. Please try again.',
      }));
    } finally {
      setLoading(false);
    }
  }, [userId, formatMessage]);

  const handleCopyAll = useCallback(() => {
    const codesText = backupCodes.join('\n');
    navigator.clipboard.writeText(codesText);
    setCopied(true);
    setTimeout(() => setCopied(false), 3000);
  }, [backupCodes]);

  const handleDownload = useCallback(() => {
    const codesText = `Mattermost MFA Backup Codes\n\nGenerated: ${new Date().toLocaleString()}\n\n${backupCodes.join('\n')}\n\nKeep these codes in a safe place. Each code can only be used once.`;
    const blob = new Blob([codesText], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'mattermost-backup-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  }, [backupCodes]);

  const handlePrint = useCallback(() => {
    window.print();
  }, []);

  if (loading) {
    return <LoadingScreen />;
  }

  if (!hasGenerated) {
    return (
      <div className='backup-codes-generate'>
        <div className='backup-codes-info'>
          <i className='icon icon-shield-check-outline' />
          <h4>
            <FormattedMessage
              id='mfa.backup_codes.title'
              defaultMessage='MFA Backup Codes'
            />
          </h4>
          <p>
            <FormattedMessage
              id='mfa.backup_codes.description'
              defaultMessage='Backup codes can be used to access your account if you lose access to your authenticator device. Each code can only be used once.'
            />
          </p>
          <button
            className='btn btn-primary'
            onClick={generateCodes}
            disabled={loading}
          >
            <FormattedMessage
              id='mfa.backup_codes.generate'
              defaultMessage='Generate Backup Codes'
            />
          </button>
          {error && (
            <div className='alert alert-danger'>
              {error}
            </div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className='backup-codes-display'>
      <div className='backup-codes-header'>
        <i className='icon icon-shield-check-outline' />
        <h4>
          <FormattedMessage
            id='mfa.backup_codes.title'
            defaultMessage='MFA Backup Codes'
          />
        </h4>
      </div>

      <div className='backup-codes-warning'>
        <i className='icon icon-alert-outline' />
        <p>
          <FormattedMessage
            id='mfa.backup_codes.warning'
            defaultMessage='Save these codes in a secure location. They will only be shown once. Each code can only be used once.'
          />
        </p>
      </div>

      <div className='backup-codes-list'>
        {backupCodes.map((code, index) => (
          <div
            key={index}
            className='backup-code-item'
          >
            <span className='code-number'>{index + 1}.</span>
            <code className='code-value'>{code}</code>
          </div>
        ))}
      </div>

      <div className='backup-codes-actions'>
        <button
          className='btn btn-primary'
          onClick={handleCopyAll}
        >
          <i className='icon icon-content-copy' />
          {copied ? (
            <FormattedMessage
              id='mfa.backup_codes.copied'
              defaultMessage='Copied!'
            />
          ) : (
            <FormattedMessage
              id='mfa.backup_codes.copy_all'
              defaultMessage='Copy All'
            />
          )}
        </button>
        <button
          className='btn btn-secondary'
          onClick={handleDownload}
        >
          <i className='icon icon-download' />
          <FormattedMessage
            id='mfa.backup_codes.download'
            defaultMessage='Download'
          />
        </button>
        <button
          className='btn btn-secondary'
          onClick={handlePrint}
        >
          <i className='icon icon-printer' />
          <FormattedMessage
            id='mfa.backup_codes.print'
            defaultMessage='Print'
          />
        </button>
      </div>

      {error && (
        <div className='alert alert-danger'>
          {error}
        </div>
      )}
    </div>
  );
};

export default BackupCodesDisplay;

