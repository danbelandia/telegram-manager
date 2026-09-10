// Hooks de datos de grupos (guia frontend seccion 4): TanStack Query
// con estados loading/error/empty. El retry es false para que el UI
// muestre el error de inmediato (el dashboard ofrece reintento manual).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteGroup, getGroup, listGroups } from './api'

export function useGroups() {
  return useQuery({
    queryKey: ['groups'],
    queryFn: listGroups,
    retry: false,
  })
}

export function useGroup(telegramId: string) {
  return useQuery({
    queryKey: ['groups', telegramId],
    queryFn: () => getGroup(telegramId),
    retry: false,
  })
}

export function useDeleteGroup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: deleteGroup,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['groups'] })
    },
  })
}