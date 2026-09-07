// Editor controlado de botones inline (slice 2): array de filas, cada
// fila un array de {text, url}. Expone agregar/quitar fila y
// agregar/quitar boton por fila. Limites cliente: 8 filas, 8 botones
// por fila, text<=64, url http(s) — la validacion final vive en el
// servicio, este componente solo evita que el cliente envie algo
// trivialmente invalido.
// Migrado a Mantine v7 (frontend-refresh slice 2 ad-hoc).
import { ActionIcon, Button, Group, Paper, Stack, Text, TextInput } from '@mantine/core'
import { IconPlus, IconTrash } from '@tabler/icons-react'
import type { InlineButton } from './types'

interface ButtonsEditorProps {
  value: InlineButton[][]
  onChange: (rows: InlineButton[][]) => void
}

function emptyButton(): InlineButton {
  return { text: '', url: '' }
}

export default function ButtonsEditor({ value, onChange }: ButtonsEditorProps) {
  const addRow = () => {
    if (value.length >= 8) return
    onChange([...value, [emptyButton()]])
  }
  const removeRow = (rowIdx: number) => {
    onChange(value.filter((_, i) => i !== rowIdx))
  }
  const addButton = (rowIdx: number) => {
    if (value[rowIdx].length >= 8) return
    onChange(value.map((row, i) => (i === rowIdx ? [...row, emptyButton()] : row)))
  }
  const removeButton = (rowIdx: number, btnIdx: number) => {
    onChange(
      value.map((row, i) =>
        i === rowIdx ? row.filter((_, j) => j !== btnIdx) : row,
      ),
    )
  }
  const updateButton = (
    rowIdx: number,
    btnIdx: number,
    field: 'text' | 'url',
    next: string,
  ) => {
    onChange(
      value.map((row, i) =>
        i === rowIdx
          ? row.map((btn, j) => (j === btnIdx ? { ...btn, [field]: next } : btn))
          : row,
      ),
    )
  }

  if (value.length === 0) {
    return (
      <Stack gap="sm">
        <Text size="sm" c="dimmed">
          Botones inline (URL): máximo 8 filas × 8 botones.
        </Text>
        <Group>
          <Button
            type="button"
            variant="default"
            leftSection={<IconPlus size={16} />}
            onClick={addRow}
          >
            + Agregar fila de botones
          </Button>
        </Group>
      </Stack>
    )
  }

  return (
    <Stack gap="sm">
      <Text size="sm" c="dimmed">
        Botones inline (URL): máximo 8 filas × 8 botones.
      </Text>

      <Stack gap="sm">
        {value.map((row, rowIdx) => (
          <Paper key={rowIdx} withBorder p="md" radius="md">
            <Stack gap="sm">
              <Group justify="space-between">
                <Text fw={600} size="sm">Fila {rowIdx + 1}</Text>
                <Button
                  type="button"
                  size="xs"
                  color="red"
                  variant="light"
                  leftSection={<IconTrash size={14} />}
                  onClick={() => removeRow(rowIdx)}
                  aria-label={`Quitar fila ${rowIdx + 1}`}
                >
                  Quitar fila
                </Button>
              </Group>

              <Stack gap="xs">
                {row.map((btn, btnIdx) => (
                  <Group key={btnIdx} gap="xs" wrap="nowrap" align="flex-start">
                    <TextInput
                      style={{ flex: 1 }}
                      placeholder="Texto del botón"
                      value={btn.text}
                      maxLength={64}
                      onChange={(e) => updateButton(rowIdx, btnIdx, 'text', e.currentTarget.value)}
                      aria-label={`Texto del botón ${rowIdx + 1}.${btnIdx + 1}`}
                      radius="md"
                    />
                    <TextInput
                      style={{ flex: 1 }}
                      type="url"
                      placeholder="https://..."
                      value={btn.url}
                      maxLength={256}
                      onChange={(e) => updateButton(rowIdx, btnIdx, 'url', e.currentTarget.value)}
                      aria-label={`URL del botón ${rowIdx + 1}.${btnIdx + 1}`}
                      radius="md"
                    />
                    <ActionIcon
                      type="button"
                      color="red"
                      variant="light"
                      onClick={() => removeButton(rowIdx, btnIdx)}
                      aria-label={`Quitar botón ${rowIdx + 1}.${btnIdx + 1}`}
                    >
                      <IconTrash size={14} />
                    </ActionIcon>
                  </Group>
                ))}
              </Stack>

              <Group>
                <Button
                  type="button"
                  size="xs"
                  variant="default"
                  leftSection={<IconPlus size={14} />}
                  onClick={() => addButton(rowIdx)}
                  disabled={row.length >= 8}
                >
                  + Agregar botón a la fila
                </Button>
              </Group>
            </Stack>
          </Paper>
        ))}
      </Stack>

      <Group>
        <Button
          type="button"
          variant="default"
          leftSection={<IconPlus size={16} />}
          onClick={addRow}
          disabled={value.length >= 8}
        >
          + Agregar fila de botones
        </Button>
      </Group>
    </Stack>
  )
}
