/** Tenant in the admin panel list. */
export interface AdminTenant {
  id: number
  slug: string
  plan: string
  status: string
  trial_ends_at: string | null
  expires_at: string | null
  max_groups: number
  max_messages_day: number
  bot_username: string | null
  created_at: string
}

/** Body for PUT /api/admin/tenants/:id */
export interface UpdateTenantLicense {
  status?: string | null
  plan?: string | null
  trial_ends_at?: string | null
  expires_at?: string | null
  max_groups?: number | null
  max_messages_day?: number | null
}
