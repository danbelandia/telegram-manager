// Hooks de datos de moderacion (guia frontend seccion 4): lecturas con
// useQuery (retry:false, el UI ofrece reintento) y acciones con
// useMutation que invalidan las queries afectadas para que los logs y
// listados se refresquen despues de cada accion (doD del MVP: la accion
// debe verse reflejada en /groups/:id/logs).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  approveJoinRequest,
  banUser,
  batchDecideJoinRequests,
  deleteMessage,
  getGroupUser,
  listGroupUsers,
  lockGroup,
  muteUser,
  pinMessage,
  rejectJoinRequest,
  unbanUser,
  unmuteUser,
  unlockGroup,
  listGroupLogs,
  listJoinRequests,
  type JoinRequestParams,
} from './api'

// Claves raiz de cada subrecurso del grupo; se anidan bajo ['groups', id]
// (decision D5) para invalidar grupos y logs de una vez.
const usersKey = (groupId: string) => ['groups', groupId, 'users'] as const
const requestsKey = (groupId: string, params?: JoinRequestParams) =>
  ['groups', groupId, 'requests', params ?? {}] as const
const logsKey = (groupId: string) => ['groups', groupId, 'logs'] as const

// ── Lecturas ──────────────────────────────────────────────────────────

export function useGroupUsers(groupId: string) {
  return useQuery({ queryKey: usersKey(groupId), queryFn: () => listGroupUsers(groupId), retry: false })
}

export function useGroupUser(groupId: string, userId: string) {
  return useQuery({
    queryKey: [...usersKey(groupId), userId],
    queryFn: () => getGroupUser(groupId, userId),
    retry: false,
    enabled: userId.length > 0,
  })
}

export function useJoinRequests(groupId: string, params?: JoinRequestParams) {
  return useQuery({
    queryKey: requestsKey(groupId, params),
    queryFn: () => listJoinRequests(groupId, params),
    retry: false,
  })
}

export function useGroupLogs(groupId: string) {
  return useQuery({ queryKey: logsKey(groupId), queryFn: () => listGroupLogs(groupId), retry: false })
}

// ── Mutaciones ────────────────────────────────────────────────────────

interface UserAction {
  groupId: string
  userId: number
}

// Banco/desbaneo y mute/unmute invalidan la lista de usuarios (cambia la
// condicion del miembro) y los logs (la accion se audita).
export function useBanUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, userId }: UserAction) => banUser(groupId, userId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: usersKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

export function useUnbanUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, userId }: UserAction) => unbanUser(groupId, userId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: usersKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

export function useMuteUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, userId }: UserAction) => muteUser(groupId, userId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: usersKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

export function useUnmuteUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, userId }: UserAction) => unmuteUser(groupId, userId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: usersKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

interface MessageAction {
  groupId: string
  messageId: number
}

// delete/pin no cambian la lista de usuarios; solo se auditan en logs.
export function useDeleteMessage() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, messageId }: MessageAction) => deleteMessage(groupId, messageId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

export function usePinMessage() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, messageId }: MessageAction) => pinMessage(groupId, messageId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

// lock/unlock cambian el estado del grupo (se invalida el detalle) y se
// auditan en logs.
export function useLockGroup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (groupId: string) => lockGroup(groupId),
    onSuccess: (_data, groupId) => {
      qc.invalidateQueries({ queryKey: ['groups', groupId] })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

export function useUnlockGroup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (groupId: string) => unlockGroup(groupId),
    onSuccess: (_data, groupId) => {
      qc.invalidateQueries({ queryKey: ['groups', groupId] })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

interface JoinRequestAction {
  groupId: string
  requestId: number
}

// approve/reject cambian el estado de la solicitud (se invalida la lista)
// y se auditan en logs.
export function useApproveJoinRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, requestId }: JoinRequestAction) =>
      approveJoinRequest(groupId, requestId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: requestsKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

export function useRejectJoinRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, requestId }: JoinRequestAction) =>
      rejectJoinRequest(groupId, requestId),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: requestsKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}

// ── Batch ────────────────────────────────────────────────────────────

interface BatchJoinRequestAction {
  groupId: string
  action: 'approve' | 'reject'
  requestIds: number[]
}

// BatchDecide procesa N solicitudes en una sola request. Invalida la
// lista de solicitudes y los logs al completar.
export function useBatchJoinRequest() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ groupId, action, requestIds }: BatchJoinRequestAction) =>
      batchDecideJoinRequests(groupId, action, requestIds),
    onSuccess: (_data, { groupId }) => {
      qc.invalidateQueries({ queryKey: requestsKey(groupId) })
      qc.invalidateQueries({ queryKey: logsKey(groupId) })
    },
  })
}