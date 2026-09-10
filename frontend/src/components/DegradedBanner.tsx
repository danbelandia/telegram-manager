// Banner de estado degradado (spec frontend-ui-foundation REQ 6, 9):
// muestra un alert cuando bot_status es disconnected o unknown.
// Polling cada 60s via GET /api/tenants/me/status. Desaparece cuando
// el bot vuelve a connected.
import { useEffect, useState } from 'react'
import { Alert } from '@mantine/core'
import { IconAlertTriangle } from '@tabler/icons-react'
import { getTenantStatus } from '../features/tenant/api'
import { ApiError } from '../lib/api-client'

export default function DegradedBanner() {
  const [status, setStatus] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setInterval> | undefined

    const check = async () => {
      try {
        const data = await getTenantStatus()
        if (!cancelled) setStatus(data.bot_status)
      } catch (err) {
        // 401 u otro error: no mostrar banner (sesion no restaurable)
        if (!cancelled && !(err instanceof ApiError && err.status === 401)) {
          setStatus('unknown')
        }
      }
    }

    // Check inmediato + polling cada 60s
    void check()
    timer = setInterval(check, 60_000)

    return () => {
      cancelled = true
      if (timer) clearInterval(timer)
    }
  }, [])

  if (!status || status === 'connected') return null

  return (
    <Alert
      variant="light"
      color="yellow"
      icon={<IconAlertTriangle size={16} />}
      mb="md"
      data-testid="degraded-banner"
    >
      {status === 'disconnected'
        ? 'El bot esta desconectado. El token puede haber sido revocado. Rota el token desde Configuracion para restaurar.'
        : 'Estado del bot desconocido. Verifica la conexion desde Configuracion.'}
    </Alert>
  )
}
