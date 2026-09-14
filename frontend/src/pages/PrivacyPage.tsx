// Politica de Privacidad publica de Sentinel (change legal-pages).
// Ruta `/privacy`, fuera de RequireAuth y de PublicOnly: cualquier
// visitante puede verla. Describe que datos se recopilan, para que se
// usan, donde se almacenan, con quien se comparten, cuanto se retienen
// y que derechos tiene el admin. Mismo lenguaje visual que TermsPage
// (Container size="md", Stack, Title, Text de Mantine).
import { Anchor, Container, Stack, Text, Title } from '@mantine/core'
import { Link } from 'react-router-dom'

export default function PrivacyPage() {
  return (
    <Container size="md" py="xl">
      <Stack gap="lg">
        <Stack gap="xs">
          <Title order={1}>Política de Privacidad</Title>
          <Text c="dimmed" size="sm">
            Última actualización: enero de 2026
          </Text>
        </Stack>

        <Stack gap="md">
          <Stack gap="xs">
            <Title order={2}>1. Datos que recopilamos</Title>
            <Text>
              Para operar el servicio almacenamos: tu email o nombre de usuario
              de administrador, el hash de tu contraseña (bcrypt), el token del
              bot de Telegram cifrado en la base de datos, los registros de
              acciones administrativas que realizás desde el panel y los
              identificadores de Telegram de los usuarios que interactúan con
              tus grupos a través del bot.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>2. Para qué usamos los datos</Title>
            <Text>
              Los datos se utilizan exclusivamente para operar Sentinel:
              autenticar administradores, ejecutar acciones de moderación
              solicitadas desde el panel y mantener un registro de auditoría
              que permita revisar quién hizo cada acción.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>3. Almacenamiento</Title>
            <Text>
              Los datos se almacenan en una base de datos PostgreSQL administrada
              por Neon. El tráfico viaja cifrado mediante HTTPS. Los tokens de
              bot se guardan cifrados en reposo utilizando AES-GCM.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>4. Compartir datos</Title>
            <Text>
              No vendemos ni compartimos datos con terceros con fines
              comerciales. Compartimos con Telegram la información estrictamente
              necesaria para operar el bot a través de su API oficial. Los
              proveedores de infraestructura (Neon para la base de datos, Render
              para el hosting) tienen acceso limitado a los datos únicamente
              para tareas de mantenimiento.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>5. Retención</Title>
            <Text>
              Los registros de acciones administrativas se conservan mientras
              tu cuenta esté activa. Podés solicitar su eliminación en
              cualquier momento escribiéndonos a través de los canales
              indicados en la sección de contacto.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>6. Tus derechos</Title>
            <Text>
              Como administrador, podés solicitar el acceso, la rectificación o
              la eliminación de los datos asociados a tu cuenta. Para ejercer
              cualquiera de estos derechos, contactanos.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>7. Cookies</Title>
            <Text>
              Sentinel utiliza una cookie httpOnly para almacenar el refresh
              token de autenticación JWT. No utilizamos cookies de seguimiento
              ni de analítica publicitaria.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>8. Cambios</Title>
            <Text>
              Podemos actualizar esta política para reflejar cambios en el
              servicio o en la plataforma de Telegram. Cuando lo hagamos, te
              avisaremos por email o mediante un aviso dentro del panel.
            </Text>
          </Stack>

          <Stack gap="xs">
            <Title order={2}>9. Contacto</Title>
            <Text>
              Para consultas sobre privacidad, escribinos a través de
              sentinelbot.pro o al canal de privacidad disponible en el panel.
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
