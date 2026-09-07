// Hooks de datos de publicaciones (guia frontend seccion 4): lectura con
// useQuery (retry:false, el UI ofrece reintento) y creacion con
// useMutation que invalida la lista para que la nueva publicacion
// aparezca de inmediato (doD: la accion debe reflejarse en el panel).
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createPublication, getPublication, listPublications } from './api'

const publicationsKey = ['publications'] as const

export function usePublications() {
  return useQuery({
    queryKey: publicationsKey,
    queryFn: listPublications,
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
      qc.invalidateQueries({ queryKey: publicationsKey })
    },
  })
}
