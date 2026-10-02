/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'

export function useUserGuideSections() {
  const { t } = useTranslation()
  return [
    {
      id: 'welcome',
      title: t('User guide'),
      markdown: t(
        'RenewAPI connects your applications to AI models through one gateway. Available models, prices, quotas and payment methods follow this site’s live configuration. Keep passwords, API keys and management access tokens private.'
      ),
    },
    {
      id: 'sign-in',
      title: t('Sign in'),
      markdown: t(
        'Use an enabled password, Passkey or OAuth login method. Accounts with two-factor authentication must also enter an authentication or backup code. Use the password recovery entry when needed. Registration is available only when enabled by the site.'
      ),
    },
    {
      id: 'quick-start',
      title: t('Quick start'),
      markdown: t(
        '1. Open the model list and copy the exact model ID.\n2. Create a dedicated API key with appropriate quota and permissions.\n3. Configure your client with this site’s API address, the key and the matching protocol.\n4. Send a short request, then check usage logs for the result and charge.'
      ),
    },
    {
      id: 'keys',
      title: t('API Keys'),
      markdown: t(
        'API keys authorize model requests. Set an expiry, quota, allowed models and group as needed. Keep separate keys for different applications. Deleting or disabling a key stops subsequent requests using that key.'
      ),
    },
    {
      id: 'wallet',
      title: t('Wallet'),
      markdown: t(
        'Recharge through a payment method offered by the site or redeem a quota code. Review the amount before confirming. After a hosted payment, check order history and your updated balance before creating another order.'
      ),
    },
    {
      id: 'subscriptions',
      title: t('Subscriptions'),
      markdown: t(
        'Plans show their actual price, quota, reset period and validity. Purchase limits are enforced by the server. The wallet lists all active subscriptions and subscription history. Billing Preference controls whether subscriptions or wallet balance are used first.'
      ),
    },
    {
      id: 'usage',
      title: t('Usage Logs'),
      markdown: t(
        'Consumption, bills and other logs have separate tabs. Drawing and task logs remain available from the usage-log menu. Ordinary accounts see their own records; administrators can switch between their records and permitted site records.'
      ),
    },
    {
      id: 'troubleshooting',
      title: t('Troubleshooting'),
      markdown: t(
        'For 401, check the key and Authorization header. For 403, check account and model permissions. For 404, verify the base URL, endpoint and model ID. For 429, reduce concurrency and retry with backoff. For server errors, retain the request ID and contact the site administrator.'
      ),
    },
  ]
}
