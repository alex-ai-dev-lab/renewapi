/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */

// UI parity does not enable SnowAPI-only backend rules. Keep unsupported
// transports out of navigation and background queries instead of faking data.
export const backendCapabilities = {
  adminPermissionCatalog: false,
  invitationCodes: false,
  ipAudit: false,
  minimalOperations: false,
  modelHealthTimeline: false,
  channelOpsSummary: false,
  subscriptionBalanceCheckout: false,
  subscriptionRollingWindow: false,
  subscriptionReset: false,
} as const
