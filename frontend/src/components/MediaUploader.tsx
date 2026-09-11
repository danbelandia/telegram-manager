// MediaUploader: Dropzone para subir fotos/videos desde el PC.
// Wrappea @mantine/dropzone con estados idle/accepting/uploading/preview.
// Props controladas: value (url+type) y onChange para que el padre
// gestione el estado. Al soltar un archivo, lo sube via POST /api/media
// y devuelve la referencia para usar en PublishRequest.
import { useRef, useState } from 'react'
import {
  Group,
  Text,
  Stack,
  Image,
  Badge,
  ActionIcon,
  Button,
  Loader,
  Alert,
} from '@mantine/core'
import { Dropzone, MIME_TYPES } from '@mantine/dropzone'
import { IconUpload, IconX, IconPhoto, IconVideo, IconFile } from '@tabler/icons-react'
import { uploadMedia } from '../features/media/api'
import type { MediaUploadResponse } from '../features/media/types'

const MAX_FILE_SIZE = 50 * 1024 * 1024 // 50 MB (límite Telegram video)

const ACCEPT_MIME_TYPES = {
  [MIME_TYPES.jpeg]: ['.jpg', '.jpeg'],
  [MIME_TYPES.png]: ['.png'],
  [MIME_TYPES.gif]: ['.gif'],
  [MIME_TYPES.webp]: ['.webp'],
  [MIME_TYPES.mp4]: ['.mp4'],
}

export interface MediaValue {
  url: string
  type: 'photo' | 'video'
  filename: string
}

interface MediaUploaderProps {
  value: MediaValue | null
  onChange: (value: MediaValue | null) => void
  onError?: (message: string) => void
  disabled?: boolean
}

export default function MediaUploader({
  value,
  onChange,
  onError,
  disabled = false,
}: MediaUploaderProps) {
  const openRef = useRef<() => void>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleDrop = async (files: File[]) => {
    const file = files[0]
    if (!file) return

    setError(null)
    setUploading(true)

    try {
      const result: MediaUploadResponse = await uploadMedia(file)
      onChange({ url: result.url, type: result.type, filename: result.filename })
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Error al subir archivo'
      setError(msg)
      onError?.(msg)
    } finally {
      setUploading(false)
    }
  }

  const handleReject = (rejections: any[]) => {
    if (rejections.length > 0) {
      const err = rejections[0]?.errors?.[0]
      if (err?.code === 'file-too-large') {
        setError('El archivo excede 50 MB.')
      } else if (err?.code === 'file-invalid-type') {
        setError('Tipo de archivo no soportado. Usá JPG, PNG, GIF, WebP o MP4.')
      } else {
        setError('Archivo no válido.')
      }
    }
  }

  const handleRemove = () => {
    onChange(null)
    setError(null)
  }

  // Preview mode: archivo ya subido
  if (value) {
    return (
      <Stack gap="xs">
        <Group justify="space-between" gap="xs">
          <Group gap="sm" wrap="nowrap">
            {value.type === 'photo' ? (
              <Image
                src={value.url}
                alt="Preview"
                w={80}
                h={60}
                fit="cover"
                radius="md"
              />
            ) : (
              <Group
                w={80}
                h={60}
                justify="center"
                align="center"
                style={{ backgroundColor: 'var(--mantine-color-gray-1)', borderRadius: 'var(--mantine-radius-md)' }}
              >
                <IconVideo size={24} color="var(--mantine-color-blue-6)" />
              </Group>
            )}
            <Stack gap={2}>
              <Text size="sm" fw={500} truncate maw={200}>
                {value.filename}
              </Text>
              <Badge
                color={value.type === 'photo' ? 'green' : 'blue'}
                variant="light"
                size="sm"
              >
                {value.type === 'photo' ? 'Foto' : 'Video'}
              </Badge>
            </Stack>
          </Group>
          <ActionIcon
            color="red"
            variant="subtle"
            onClick={handleRemove}
            disabled={disabled}
          >
            <IconX size={16} />
          </ActionIcon>
        </Group>
      </Stack>
    )
  }

  // Upload mode: Dropzone
  return (
    <Stack gap="xs">
      <Dropzone
        openRef={openRef}
        onDrop={handleDrop}
        onReject={handleReject}
        maxSize={MAX_FILE_SIZE}
        accept={ACCEPT_MIME_TYPES}
        disabled={disabled || uploading}
        loading={uploading}
        radius="md"
        style={{
          borderStyle: 'dashed',
          borderWidth: 2,
          backgroundColor: 'var(--mantine-color-gray-0)',
        }}
      >
        <Group justify="center" gap="xl" mih={120} style={{ pointerEvents: 'none' }}>
          <Dropzone.Accept>
            <IconUpload size={40} color="var(--mantine-color-blue-6)" />
          </Dropzone.Accept>
          <Dropzone.Reject>
            <IconX size={40} color="var(--mantine-color-red-6)" />
          </Dropzone.Reject>
          <Dropzone.Idle>
            <IconPhoto size={40} color="var(--mantine-color-dimmed)" />
          </Dropzone.Idle>

          <Stack gap={4} align="center">
            <Text size="xl" inline>
              Arrastrá un archivo acá
            </Text>
            <Text size="sm" c="dimmed" inline>
              JPG, PNG, GIF, WebP (≤ 10 MB) o MP4 (≤ 50 MB)
            </Text>
            <Button
              variant="light"
              size="sm"
              mt={4}
              leftSection={<IconFile size={14} />}
              onClick={(e) => {
                e.stopPropagation()
                openRef.current?.()
              }}
              style={{ pointerEvents: 'all' }}
            >
              Seleccionar archivo
            </Button>
          </Stack>
        </Group>
      </Dropzone>

      {uploading && (
        <Group gap="xs">
          <Loader size="xs" />
          <Text size="sm" c="dimmed">Subiendo archivo…</Text>
        </Group>
      )}

      {error && (
        <Alert color="red" variant="light">
          {error}
        </Alert>
      )}
    </Stack>
  )
}
