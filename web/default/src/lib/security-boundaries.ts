/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

export const MAX_CREDENTIAL_FILE_BYTES = 1024 * 1024
export const MAX_CREDENTIAL_FILE_COUNT = 100

type FileMetadata = Pick<File, 'name' | 'size' | 'type'>

export type CredentialFileValidation =
  | { valid: true }
  | { valid: false; reason: 'count' | 'size' | 'type'; fileName?: string }

export function validateCredentialFiles(
  files: FileMetadata[]
): CredentialFileValidation {
  if (files.length > MAX_CREDENTIAL_FILE_COUNT) {
    return { valid: false, reason: 'count' }
  }
  for (const file of files) {
    if (file.size > MAX_CREDENTIAL_FILE_BYTES) {
      return { valid: false, reason: 'size', fileName: file.name }
    }
    const hasJSONExtension = file.name.toLowerCase().endsWith('.json')
    const hasJSONMimeType = !file.type || file.type === 'application/json'
    if (!hasJSONExtension || !hasJSONMimeType) {
      return { valid: false, reason: 'type', fileName: file.name }
    }
  }
  return { valid: true }
}

export function sanitizeDownloadFilename(
  filename: string,
  fallback = 'download.txt'
): string {
  const withoutControls = Array.from(filename)
    .filter((character) => {
      const codePoint = character.codePointAt(0) ?? 0
      return codePoint > 0x1f && codePoint !== 0x7f
    })
    .join('')
  const cleaned = withoutControls
    .replace(/[<>:"/\\|?*]/g, '_')
    .replace(/[. ]+$/g, '')
    .trim()
    .slice(0, 120)
  return cleaned || fallback
}
