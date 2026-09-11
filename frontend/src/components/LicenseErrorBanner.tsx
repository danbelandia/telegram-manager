import { Alert } from '@mantine/core'
import { IconLock } from '@tabler/icons-react'

interface LicenseErrorBannerProps {
  code: 'LICENSE_SUSPENDED' | 'LICENSE_EXPIRED'
}

export default function LicenseErrorBanner({ code }: LicenseErrorBannerProps) {
  return (
    <Alert
      variant="light"
      color="red"
      icon={<IconLock size={16} />}
      mb="md"
      data-testid="license-error-banner"
    >
      {code === 'LICENSE_SUSPENDED'
        ? 'Tu licencia esta suspendida. Contacta al administrador para reactivarla.'
        : 'Tu licencia ha expirado. Contacta al administrador para continuar.'}
    </Alert>
  )
}
