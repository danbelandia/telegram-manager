// Hooks de datos de moderacion automatica (Fase 3, slice 2 + slice 3):
//   - Lecturas con useQuery (retry:false).
//   - Mutaciones con useMutation que invalidan la query afectada.
// Patrón consistente con features/publications y features/moderation
// (query keys anidadas bajo ['automation', subrecurso, groupId]).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  addAllowlistEntry,
  addBannedWord,
  getAutomationSettings,
  getStats,
  listBannedWords,
  listLinkAllowlist,
  listWarnings,
  putAutomationSettings,
  removeAllowlistEntry,
  removeBannedWord,
  resetWarning,
} from './api'
import type {
  AutomationSettingsUpdate,
  BannedWordRequest,
  LinkAllowlistRequest,
  StatsPeriod,
} from './types'

const settingsKey = (groupId: number) => ['automation', 'settings', groupId] as const
const bannedKey = (groupId: number) => ['automation', 'banned-words', groupId] as const
const allowlistKey = (groupId: number) => ['automation', 'link-allowlist', groupId] as const
// Slice 3 — Warnings Dashboard.
const warningsKey = (groupId: number) => ['automation', 'warnings', groupId] as const
const statsKey = (groupId: number, period: StatsPeriod) =>
  ['automation', 'stats', groupId, period] as const

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

// Slice 3 — Warnings Dashboard (Fase 3): usa useQuery sin
// refetchInterval (YAGNI). El caller dispara refresh on-demand via
// queryClient.invalidateQueries desde el botón "Refrescar".
export function useWarnings(groupId: number) {
  return useQuery({
    queryKey: warningsKey(groupId),
    queryFn: () => listWarnings(groupId),
    retry: false,
  })
}

export function useStats(groupId: number, period: StatsPeriod) {
  return useQuery({
    queryKey: statsKey(groupId, period),
    queryFn: () => getStats(groupId, period),
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

// Slice 3: el reset invalida warnings (la fila desaparece de la tabla)
// + todas las stats (no son estrictamente necesarias pero mantienen
// coherencia con el patrón de "reset refresca todo el dashboard").
export function useResetWarning(groupId: number) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: number) => resetWarning(groupId, userId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: warningsKey(groupId) })
      qc.invalidateQueries({ queryKey: ['automation', 'stats', groupId] })
    },
  })
}
