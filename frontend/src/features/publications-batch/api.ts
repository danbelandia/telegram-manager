// Cliente HTTP del endpoint batch (publications-batch change). Reusa el
// api-client central (lib/api-client.ts) para manejar el envelope,
// access token en memoria y refresh transparente de 401 (AGENTS 17.1).
import { request } from '../../lib/api-client'
import type { BatchRequest, BatchResponse } from './types'

/**
 * POST /api/publications/batch — crea hasta 10 publicaciones en una
 * sola request. Devuelve {created[], failed[]} con failure-isolation
 * per-item (status 200 OK siempre que la envelope sea valida; 400
 * solo por errores de envelope como cap excedido o JSON malformado).
 */
export function createPublicationBatch(input: BatchRequest): Promise<BatchResponse> {
  return request<BatchResponse>('/api/publications/batch', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
