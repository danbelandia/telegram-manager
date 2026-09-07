// Hooks de datos de grupos (guia frontend seccion 4): TanStack Query
// con estados loading/error/empty. El retry es false para que el UI
// muestre el error de inmediato (el dashboard ofrece reintento manual).
import { useQuery } from '@tanstack/react-query'
import { getGroup, listGroups } from './api'

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