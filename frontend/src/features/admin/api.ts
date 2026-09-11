import { request } from '../../lib/api-client'
import type { AdminTenant, UpdateTenantLicense } from './types'

/** GET /api/admin/tenants — list all tenants (super-admin). */
export function adminListTenants(): Promise<AdminTenant[]> {
  return request<AdminTenant[]>('/api/admin/tenants')
}

/** GET /api/admin/tenants/:id — tenant detail. */
export function adminGetTenant(id: number): Promise<AdminTenant> {
  return request<AdminTenant>(`/api/admin/tenants/${id}`)
}

/** PUT /api/admin/tenants/:id — update license fields. */
export function adminUpdateTenant(id: number, body: UpdateTenantLicense): Promise<{ status: string }> {
  return request<{ status: string }>(`/api/admin/tenants/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}
