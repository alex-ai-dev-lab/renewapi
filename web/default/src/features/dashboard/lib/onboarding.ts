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

/**
 * Fresh-account onboarding is deliberately data-driven: a step may only link
 * to a destination that is actually visible for the current role/module
 * configuration. Callers pass the visibility of each destination, and steps
 * without a visible destination fall back to guidance text — never a dead or
 * disabled-module link.
 */
export type OnboardingUrl = '/wallet' | '/keys' | '/model-list' | '/docs'

export type OnboardingStepId = 'credit' | 'api-key' | 'model' | 'docs'

export type OnboardingStep = {
  id: OnboardingStepId
  titleKey: string
  descriptionKey: string
  url?: OnboardingUrl
  linkKey?: string
}

export type OnboardingAvailability = {
  wallet: boolean
  keys: boolean
  models: boolean
  docs?: boolean
}

export function buildOnboardingSteps(
  availability: OnboardingAvailability
): OnboardingStep[] {
  const steps: OnboardingStep[] = [
    {
      id: 'credit',
      titleKey: 'Confirm your available credit',
      descriptionKey: availability.wallet
        ? 'Check your balance in Wallet and add funds if you need more.'
        : 'If you need credit, contact an administrator.',
      ...(availability.wallet
        ? { url: '/wallet' as const, linkKey: 'Open Wallet' }
        : {}),
    },
  ]

  if (availability.keys) {
    steps.push({
      id: 'api-key',
      titleKey: 'Create an API key',
      descriptionKey: 'Create a key to authenticate your requests.',
      url: '/keys',
      linkKey: 'Create key',
    })
  }

  if (availability.models) {
    steps.push({
      id: 'model',
      titleKey: 'Choose a model',
      descriptionKey: 'Browse the model catalog to find an available model.',
      url: '/model-list',
      linkKey: 'Browse models',
    })
  }

  steps.push({
    id: 'docs',
    titleKey: 'Read the integration guide',
    descriptionKey:
      availability.docs === false
        ? 'Ask an administrator for integration instructions.'
        : 'Read the API connection instructions and request examples.',
    ...(availability.docs === false
      ? {}
      : { url: '/docs' as const, linkKey: 'Open guide' }),
  })

  return steps
}

/**
 * A fresh account has completed a successful data load and has neither usage
 * records nor any historical requests. The `loaded` flag prevents onboarding
 * from flashing before the empty response is confirmed.
 */
export function isFreshAccount(input: {
  loaded: boolean
  usageCount: number
  requestCount: number
}): boolean {
  if (!input.loaded) return false
  return input.usageCount === 0 && input.requestCount === 0
}
