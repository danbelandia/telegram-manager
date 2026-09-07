// Hooks de datos de publicaciones (guia frontend seccion 4): lectura con
// useQuery (retry:false, el UI ofrece reintento) y creacion con
// useMutation que invalida la lista para que la nueva publicacion
// aparezca de inmediato.
//
// Slice 2: la query key es `['publications', group_id ?? 'all']` — el
// cambio de filtro recarga la lista (no es la misma key). La mutacion
// invalida todas las variantes (prefix match: cualquier ['publications', ...]).
//
// Slice 3: la query key incluye tambien limit/offset (paginacion) y
// agrega `useCancelPublication` (DELETE).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { cancelPublication, createPublication, getPublication, listPublications } from './api'
import type { PublicationsFilter } from './types'

const publicationsKey = ['publications'] as const

export function usePublications(filter?: PublicationsFilter) {
  return useQuery({
    queryKey: [
      ...publicationsKey,
      filter?.group_id ?? 'all',
      filter?.limit ?? 50,
      filter?.offset ?? 0,
    ] as const,
    queryFn: () => listPublications(filter),
    retry: false,
  })
}

export function usePublication(id: number) {
  return useQuery({
    queryKey: [...publicationsKey, id],
    queryFn: () => getPublication(id),
    retry: false,
    enabled: Number.isFinite(id) && id > 0,
  })
}

export function useCreatePublication() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: createPublication,
    onSuccess: () => {
      // Invalidar todas las variantes (Todas / filtros por grupo) para
      // que la nueva publicacion aparezca independientemente del filtro
      // actual del admin.
      qc.invalidateQueries({ queryKey: publicationsKey })
    },
  })
}

export function useCancelPublication() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: cancelPublication,
    onSuccess: () => {
      // Tras cancelar, la fila desaparece de la tabla. Invalidamos
      // todas las variantes (filtro + paginacion) para que el UI
      // refresque automaticamente.
      qc.invalidateQueries({ queryKey: publicationsKey })
    },
  })
}
