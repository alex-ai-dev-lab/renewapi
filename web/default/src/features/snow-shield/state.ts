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
import { create } from 'zustand'

// Memory only. This is a UI hint, not authorization; the server checks its own
// signed HttpOnly clearance cookie on every protected request.
export const useSnowShieldState = create<{
  verified: boolean
  expiresAt: number
  setVerified: (verified: boolean, expiresIn?: number) => void
}>((set) => ({
  verified: false,
  expiresAt: 0,
  setVerified: (verified, expiresIn = 0) =>
    set({
      verified: verified && expiresIn > 0,
      expiresAt: verified ? Date.now() + expiresIn * 1000 : 0,
    }),
}))

export const SNOW_SHIELD_REQUIRED_EVENT = 'snowapi:shield-required'
