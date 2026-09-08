// Hook react-query para POST /api/publications/batch (publications-batch
// change). Invalida el cache de ['publications'] al exito para que la
// lista del historial refresque automaticamente (las nuevas filas se
// muestran sin reload manual). En error notifica via notifyError para
// mantener consistencia con el resto de hooks del modulo.
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { notifyError } from '../../lib/notifications'
import { createPublicationBatch } from './api'
import type { BatchRequest, BatchResponse } from './types'

/**
 * useCreatePublicationBatch — react-query mutation que crea hasta 10
 * publicaciones en una sola request. El modal (BatchWizard) consume
 * este hook para submit + retry de fallidas.
 *
 * Cache: al exito invalida el prefijo ['publications'] (igual que
 * useCreatePublication de features/publications/hooks.ts: cualquier
 * filtro/paginacion refresca con la nueva fila).
 *
 * Error handling: el 4xx/5xx lo mapea api-client a ApiError con el
 * code de seccion 18; notifyError muestra el mensaje legible al admin.
 * El body de 200 OK con failed[] no es un "error" a nivel HTTP — el
 * hook resuelve con BatchResponse completo y el modal renderiza los
 * banners verde/rojo segun corresponda.
 */
export function useCreatePublicationBatch() {
  const qc = useQueryClient()
  return useMutation<BatchResponse, Error, BatchRequest>({
    mutationFn: createPublicationBatch,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['publications'] })
    },
    onError: (err) => {
      notifyError(err.message || 'No se pudo crear el batch de publicaciones.')
    },
  })
}
