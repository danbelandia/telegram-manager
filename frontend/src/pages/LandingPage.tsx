// Landing publica `/` — hero section con feature list + CTA.
// Layout inspirado en Mantine HeroBullets: titulos a la izquierda,
// features con check icons, dos botones (Crear cuenta / Iniciar sesion).
// Responsive: en mobile apila verticalmente y oculta la imagen.
import {
  Button,
  Container,
  Image,
  List,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from '@mantine/core'
import { IconCheck } from '@tabler/icons-react'
import { Link } from 'react-router-dom'
import heroSvg from './hero-illustration.svg'
import classes from './LandingPage.module.css'

export default function LandingPage() {
  return (
    <Container size="lg">
      <div className={classes.inner}>
        <div className={classes.content}>
          <Title className={classes.title}>
            Administrá tus grupos de{' '}
            <span className={classes.highlight}>Telegram</span>
          </Title>

          <Text className={classes.description} c="dimmed" mt="md">
            Un solo panel para moderar usuarios, aprobar solicitudes de ingreso,
            programar publicaciones y auditar cada acción administrativa.
          </Text>

          <List
            className={classes.featureList}
            spacing="sm"
            size="md"
            icon={
              <ThemeIcon size={24} radius="xl" variant="light" color="blue">
                <IconCheck size={14} stroke={2} />
              </ThemeIcon>
            }
          >
            <List.Item>
              <b>Moderación con un clic</b> — banear, mutear, expulsar
              usuarios y eliminar mensajes desde el panel.
            </List.Item>
            <List.Item>
              <b>Solicitudes de ingreso</b> — aceptá o rechazá nuevos
              miembros sin abrir Telegram.
            </List.Item>
            <List.Item>
              <b>Publicaciones programadas</b> — escribí el mensaje, elegí
              la fecha y dejá que el bot lo publique por vos.
            </List.Item>
            <List.Item>
              <b>Auditoría completa</b> — cada acción queda registrada con
              quién la hizo, cuándo y el resultado.
            </List.Item>
          </List>

          <Stack className={classes.controls} gap="sm">
            <Button
              component={Link}
              to="/signup"
              radius="xl"
              size="lg"
              variant="gradient"
              gradient={{ from: 'blue', to: 'cyan', deg: 135 }}
            >
              Crear cuenta gratis
            </Button>
            <Button
              component={Link}
              to="/login"
              radius="xl"
              size="lg"
              variant="default"
            >
              Iniciar sesión
            </Button>
          </Stack>
        </div>

        <Image
          src={heroSvg}
          className={classes.heroImage}
          alt="Telegram Manager"
          w={340}
          h={340}
        />
      </div>
    </Container>
  )
}
