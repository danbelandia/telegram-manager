// Landing publica `/` (change public-signup-landing, spec
// frontend-routing REQ public-landing): presentacion del producto en
// es-AR con enlaces visibles a /signup y /login. Sin sesion requerida;
// `PublicOnly` redirige a /dashboard cuando ya hay sesion.
import { Button, Center, Group, List, Paper, Stack, Text, Title } from '@mantine/core'
import { Link } from 'react-router-dom'

export default function LandingPage() {
  return (
    <Center mih="100vh" px="md">
      <Paper withBorder shadow="md" p="xl" radius="md" w={480}>
        <Stack gap="md">
          <Title order={1} ta="center">
            Telegram Manager
          </Title>
          <Text ta="center" c="dimmed">
            Administrá tus grupos de Telegram desde un solo panel: moderación,
            solicitudes de ingreso y publicaciones programadas.
          </Text>
          <List spacing="xs" size="sm" center>
            <List.Item>Moderá usuarios y mensajes con un clic</List.Item>
            <List.Item>Aprobá solicitudes de ingreso sin abrir Telegram</List.Item>
            <List.Item>Auditoría completa de cada acción administrativa</List.Item>
          </List>
          <Group justify="center" gap="sm">
            <Button component={Link} to="/signup">
              Crear cuenta
            </Button>
            <Button component={Link} to="/login" variant="default">
              Iniciar sesión
            </Button>
          </Group>
        </Stack>
      </Paper>
    </Center>
  )
}
