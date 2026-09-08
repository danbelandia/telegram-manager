// Hooks de datos de moderacion automatica (Fase 3, slice 2):
//   - Lecturas con useQuery (retry:false).
//   - Mutaciones con useMutation que invalidan la query afectada.
// Patrón consistente con features/publications y features/moderation
// (query keys anidadas bajo ['automation', subrecurso, groupId]).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  addAllowlistEntry,
  addBannedWord,
  getAutomationSettings,
  listBannedWords,
  listLinkAllowlist,
  putAutomationSettings,
  removeAllowlistEntry,
  removeBannedWord,
} from './api'
import type {
  AutomationSettingsUpdate,
  BannedWordRequest,
  LinkAllowlistRequest,
} from './types'

const settingsKey = (groupId: number) => ['automation', 'settings', groupId] as const
const bannedKey = (groupId: number) => ['automation', 'banned-words', groupId] as const
const allowlistKey = (groupId: number) => ['automation', 'link-allowlist', groupId] as const

// ── Lecturas ──────────────────────────────────────────────────────────

export function useAutomationSettings(groupId: number) {
  return useQuery({
    queryKey: settingsKey(groupId),
    queryFn: () => getAutomationSettings(groupId),
    retry: false,
  })
}

export function useBannedWords(groupId: number) {
  return useQuery({
    queryKey: bannedKey(groupId),
    queryFn: () => listBannedWords(groupId),
    retry: false,
  })
}

export function useLinkAllowlist(groupId: number) {
  return useQuery({
    queryKey: allowlistKey(groupId),
    queryFn: () => listLinkAllowlist(groupId),
    retry: false,
  })
}

// ── Mutaciones ────────────────────────────────────────────────────────

export function useUpdateSettings(groupId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (update: AutomationSettingsUpdate) => putAutomationSettings(groupId, update),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: settingsKey(groupId) })
    },
  })
}

export function useAddBannedWord(groupId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: BannedWordRequest) => addBannedWord(groupId, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bannedKey(groupId) })
    },
  })
}

export function useRemoveBannedWord(groupId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (word: string) => removeBannedWord(groupId, word),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: bannedKey(groupId) })
    },
  })
}

export function useAddAllowlist(groupId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: LinkAllowlistRequest) => addAllowlistEntry(groupId, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: allowlistKey(groupId) })
    },
  })
}

export function useRemoveAllowlist(groupId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (domain: string) => removeAllowlistEntry(groupId, domain),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: allowlistKey(groupId) })
    },
  })
}
