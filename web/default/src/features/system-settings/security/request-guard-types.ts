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
export type RequestGuardMode = 'off' | 'observe' | 'enforce'
export type RequestGuardFailurePolicy = 'closed' | 'open'
export type RequestGuardCodec = 'qwen3guard' | 'json_policy'
export type RequestGuardProxyPolicy = 'disabled' | 'environment' | 'explicit'

export type RequestGuardScope = {
  all_groups: boolean
  groups: string[]
  models: string[]
  protocols: string[]
}

export type RequestGuardBulkhead = {
  max_concurrent: number
  max_per_endpoint: number
}

export type RequestGuardObserve = {
  worker_count: number
  queue_capacity: number
}

export type RequestGuardEndpoint = {
  id: string
  enabled: boolean
  priority: number
  base_url: string
  model: string
  codec: RequestGuardCodec
  timeout_ms: number
  input_limit_runes: number
  allow_private_ip: boolean
  proxy_policy: RequestGuardProxyPolicy
  proxy_url?: string
  has_secret: boolean
  secret_status: 'configured' | 'not_configured'
  secret?: string
  clear_secret?: boolean
}

export type RequestGuardConfig = {
  enabled: boolean
  mode: RequestGuardMode
  failure_policy: RequestGuardFailurePolicy
  input_mode: 'full_client_controlled'
  max_input_runes: number
  evaluation_timeout_ms: number
  scope: RequestGuardScope
  bulkhead: RequestGuardBulkhead
  observe: RequestGuardObserve
  store_pass_events: boolean
  store_redacted_preview: boolean
  endpoints: RequestGuardEndpoint[]
}

export type RequestGuardMetrics = {
  decisions: number
  observe_dropped: number
  fail_open: number
  audit_errors: number
  queue_depth: number
  workers: number
  bulkhead_active: number
  failovers: number
  bulkhead_rejected: number
  input_truncated: number
}

export type RequestGuardEndpointStatus = {
  endpoint_id: string
  healthy: boolean
  last_outcome: string
  last_error?: string
  last_latency_ms: number
  last_checked_at: number
}

export type RequestGuardStatus = {
  enabled: boolean
  mode: RequestGuardMode
  failure_policy: RequestGuardFailurePolicy
  metrics: RequestGuardMetrics
  endpoints: RequestGuardEndpointStatus[]
}

export type RequestGuardProbeResult = {
  reachable: boolean
  http_status: number
  codec_valid: boolean
  latency_ms: number
  model: string
  error_class?: string
  decision: string
  reason_code: string
}
