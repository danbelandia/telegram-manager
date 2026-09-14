// Terminos de Servicio publicos de Sentinel (change legal-pages). Ruta
// `/terms`, fuera de RequireAuth y de PublicOnly: cualquier visitante
// (autenticado o no) puede verlos, igual que /privacy. Mantiene el
// mismo lenguaje visual que el resto del panel: Container size="md",
// Stack, Title, Text de Mantine. El link al final usa React Router
// para volver al landing sin recargar.
import { Anchor, Container, Stack, Text, Title } from '@mantine/core'
import { Link } from 'react-router-dom'

export default function TermsPage() {
  return (
    <Container size="md" py="xl">
      <Stack gap="lg">
        <Stack gap="xs">
          <Title order={1}>Términos de Servicio</Title>
          <Text c="dimmed" size="sm">
            Última actualización: enero de 2026
          </Text>
        </Stack>

        <Stack gap="md">
          <Stack gap="xs">
            <Title order={2}>1. Aceptación</Title>
            <Text>
              Al crear una cuenta o utilizar Sentinel de cualquier forma, aceptás
              estos términos en su totalidad. Si no estás de acuerdo con alguna
              parte, no uses el servicio.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>2. Descripción del servicio</Title>
            <Text>
              Sentinel es un panel de moderación para grupos de Telegram
              operado a través de un bot. Permite administrar usuarios, aprobar
              solicitudes de ingreso, programar publicaciones y registrar cada
              acción administrativa realizada.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>3. Cuenta y responsabilidades</Title>
            <Text>
              El administrador de la cuenta es responsable de mantener la
              confidencialidad de sus credenciales y del token del bot de
              Telegram que conecta al servicio. Todas las acciones que el bot
              ejecute en tus grupos se consideran realizadas bajo tu
              autorización.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>4. Uso aceptable</Title>
            <Text>
              No está permitido utilizar Sentinel para enviar spam, acosar a
              usuarios, coordinar actividades ilegales en Telegram ni infringir
              los Términos de Servicio de la propia plataforma de Telegram.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>5. Terminación</Title>
            <Text>
              Podemos suspender o eliminar cuentas que violen estos términos,
              que abusen del servicio o que pongan en riesgo la integridad de
              Sentinel o de Telegram.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>6. Limitación de responsabilidad</Title>
            <Text>
              Sentinel se ofrece &quot;tal cual&quot;, sin garantías expresas o
              implícitas. No somos responsables de las acciones que el bot
              ejecute en tus grupos de Telegram, ni de las consecuencias de
              dichas acciones.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>7. Cambios</Title>
            <Text>
              Podemos actualizar estos términos para reflejar cambios en el
              servicio o en la plataforma de Telegram. Cuando lo hagamos, te
              avisaremos por email o mediante un aviso dentro del panel.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>8. Contacto</Title>
            <Text>
              Ante cualquier duda sobre estos términos, escribinos a través de
              sentinelbot.pro o al canal de soporte disponible en el panel.
            </Text>
          </Stack>
        </Stack>

        <Anchor component={Link} to="/" mt="md">
          ← Volver al inicio
        </Anchor>
      </Stack>
    </Container>
  )
}
