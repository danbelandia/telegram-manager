// Pagina /groups/:id/automation: editor de settings + listas de
// moderacion automatica (Fase 3, slice 2). Migrado a Mantine v7.
//
// Layout (4 secciones):
//   1. Switch principal "Habilitar moderación automática".
//   2. 4 sub-toggles (Flood, Anti-spam, Anti-link, Banned-words).
//   3. 6 NumberInputs para los thresholds (flood messages/seconds,
//      warning_limit, automute warnings/minutes, autoban warnings,
//      warning_expire_days).
//   4. 2 TagsInput para banned_words y link_allowlist.
//
// Single Save button al fondo dispara Promise.all en paralelo:
//   - PUT settings
//   - POST / DELETE por cada palabra/dominio cambiado.
// Errores por sección se acumulan como notifyError separados; un fallo
// NO aborta el resto.
import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Alert,
  Anchor,
  Button,
  Group,
  Loader,
  NumberInput,
  Paper,
  Stack,
  Switch,
  TagsInput,
  Text,
  Textarea,
  Title,
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import {
  formatAutomationError,
  validateAllowlistClient,
  validateBannedWordClient,
  validateThresholdsClient,
} from '../features/automation/error'
import {
  useAddAllowlist,
  useAddBannedWord,
  useAutomationSettings,
  useBannedWords,
  useLinkAllowlist,
  useRemoveAllowlist,
  useRemoveBannedWord,
  useUpdateSettings,
} from '../features/automation/hooks'
import { AUTOMATION_DEFAULTS, type AutomationSettings } from '../features/automation/types'

interface DraftSettings extends Omit<AutomationSettings, 'updated_at'> {}

function withDefaults(groupId: number, partial?: Partial<AutomationSettings>): DraftSettings {
  return {
    group_id: groupId,
    ...AUTOMATION_DEFAULTS,
    ...(partial ?? {}),
  }
}

/** Default pre-mute template del backend (automation/templates.go).
 * Lo mostramos como placeholder del Textarea para que el admin sepa
 * que puede customizarlo y que variables puede usar. */
const DEFAULT_PRE_MUTE_PLACEHOLDER =
  '⚠️ {nombre}, llevás {count} advertencias. Si seguís, serás silenciado por {mute_minutes} min.'

export default function GroupAutomationPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = Number(id)

  const settingsQuery = useAutomationSettings(groupId)
  const bannedQuery = useBannedWords(groupId)
  const allowlistQuery = useLinkAllowlist(groupId)

  const updateSettings = useUpdateSettings(groupId)
  const addWord = useAddBannedWord(groupId)
  const removeWord = useRemoveBannedWord(groupId)
  const addAllow = useAddAllowlist(groupId)
  const removeAllow = useRemoveAllowlist(groupId)

  // Snapshot "original" de los 3 recursos (settings, banned,
  // allowlist) para calcular el diff al guardar.
  const [originalSettings, setOriginalSettings] = useState<DraftSettings | null>(null)
  const [originalBanned, setOriginalBanned] = useState<string[]>([])
  const [originalAllowlist, setOriginalAllowlist] = useState<string[]>([])

  // Draft state local — los cambios del form no van a la API hasta el
  // Save explicito. Mantenemos referencias a los 3 recursos y
  // recalculamos diff en handleSave.
  const [draftSettings, setDraftSettings] = useState<DraftSettings | null>(null)
  const [draftBanned, setDraftBanned] = useState<string[]>([])
  const [draftAllowlist, setDraftAllowlist] = useState<string[]>([])
  const [pendingWords, setPendingWords] = useState<string[]>([])
  const [pendingAllowlist, setPendingAllowlist] = useState<string[]>([])

  // Cargar originales + draft cuando el query resuelve.
  useEffect(() => {
    if (settingsQuery.data && !originalSettings) {
      const def = withDefaults(groupId, settingsQuery.data)
      setOriginalSettings(def)
      setDraftSettings(def)
    }
  }, [settingsQuery.data, originalSettings, groupId])

  useEffect(() => {
    if (bannedQuery.data && originalBanned.length === 0) {
      setOriginalBanned(bannedQuery.data.words)
      setDraftBanned(bannedQuery.data.words)
    }
  }, [bannedQuery.data, originalBanned.length])

  useEffect(() => {
    if (allowlistQuery.data && originalAllowlist.length === 0) {
      setOriginalAllowlist(allowlistQuery.data.domains)
      setDraftAllowlist(allowlistQuery.data.domains)
      setPendingAllowlist(allowlistQuery.data.domains)
    }
  }, [allowlistQuery.data, originalAllowlist.length])

  const isLoading =
    settingsQuery.isPending || bannedQuery.isPending || allowlistQuery.isPending
  const isError = settingsQuery.isError || bannedQuery.isError || allowlistQuery.isError
  const isSaving =
    updateSettings.isPending ||
    addWord.isPending ||
    removeWord.isPending ||
    addAllow.isPending ||
    removeAllow.isPending

  const dirty = useMemo(() => {
    if (!draftSettings || !originalSettings) return false
    const settingsDirty = (Object.keys(AUTOMATION_DEFAULTS) as Array<keyof DraftSettings>).some(
      (k) => draftSettings[k] !== originalSettings[k],
    )
    const bannedDirty =
      draftBanned.length !== originalBanned.length ||
      draftBanned.some((w, i) => w !== originalBanned[i])
    const allowDirty =
      draftAllowlist.length !== originalAllowlist.length ||
      draftAllowlist.some((d, i) => d !== originalAllowlist[i])
    return settingsDirty || bannedDirty || allowDirty
  }, [draftSettings, originalSettings, draftBanned, originalBanned, draftAllowlist, originalAllowlist])

  if (isLoading) {
    return (
      <Stack gap="md">
        <Loader size="sm" />
        <Text size="sm" c="dimmed">Cargando configuración de moderación automática…</Text>
      </Stack>
    )
  }

  if (isError || !draftSettings) {
    return (
      <Stack gap="md">
        <Title order={2}>Moderación automática</Title>
        <Alert color="red" variant="light">
          {formatAutomationError(
            settingsQuery.error ?? bannedQuery.error ?? allowlistQuery.error,
          )}
        </Alert>
        <Group>
          <Button variant="default" onClick={() => settingsQuery.refetch()}>
            Reintentar
          </Button>
        </Group>
      </Stack>
    )
  }

  const updateDraft = <K extends keyof DraftSettings>(key: K, value: DraftSettings[K]) => {
    setDraftSettings((cur) => (cur ? { ...cur, [key]: value } : cur))
  }

  const handleAddWord = (word: string) => {
    const validation = validateBannedWordClient(word)
    if (validation) {
      notifications.show({ color: 'red', title: 'Palabra inválida', message: validation })
      return
    }
    const normalized = word.trim().toLowerCase()
    if (draftBanned.includes(normalized)) return
    setDraftBanned((cur) => [...cur, normalized])
    setPendingWords((cur) => [...cur, normalized])
  }

  const handleAddDomain = (domain: string) => {
    const validation = validateAllowlistClient(domain)
    if (validation) {
      notifications.show({ color: 'red', title: 'Dominio inválido', message: validation })
      return
    }
    const normalized = domain.trim()
    if (draftAllowlist.includes(normalized)) return
    setDraftAllowlist((cur) => [...cur, normalized])
    setPendingAllowlist((cur) => [...cur, normalized])
  }

  const handleSave = async () => {
    if (!draftSettings) return
    const validation = validateThresholdsClient(draftSettings)
    if (validation) {
      notifications.show({ color: 'red', title: 'Thresholds inválidos', message: validation })
      return
    }

    // 1. Diff settings: si el draft difiere del original, PUT el delta.
    const settingsDelta: Partial<DraftSettings> = {}
    for (const key of Object.keys(AUTOMATION_DEFAULTS) as Array<keyof DraftSettings>) {
      if (draftSettings[key] !== originalSettings![key]) {
        ;(settingsDelta as Record<string, unknown>)[key] = draftSettings[key]
      }
    }

    // 2. Diff banned words: agregar las nuevas, eliminar las removidas.
    const wordsToAdd = draftBanned.filter((w) => !originalBanned.includes(w))
    const wordsToRemove = originalBanned.filter((w) => !draftBanned.includes(w))

    // 3. Diff link allowlist: misma logica.
    const domainsToAdd = draftAllowlist.filter((d) => !originalAllowlist.includes(d))
    const domainsToRemove = originalAllowlist.filter((d) => !draftAllowlist.includes(d))

    // Promise.all paralelo: cada call devuelve su error individual si
    // falla. Errores se acumulan como notifyError y NO abortan el
    // resto (el admin puede ver que falló y reintentar solo esa
    // sección).
    const tasks: Array<Promise<unknown>> = []

    if (Object.keys(settingsDelta).length > 0) {
      // Wrap en catch para que Promise.all no aborte con el primer
      // rechazo: queremos acumular TODOS los errores y mostrarlos
      // como notificaciones separadas.
      tasks.push(updateSettings.mutateAsync(settingsDelta).catch((e) => e))
    }
    for (const w of wordsToAdd) {
      tasks.push(addWord.mutateAsync({ word: w }).catch((e) => e))
    }
    for (const w of wordsToRemove) {
      tasks.push(removeWord.mutateAsync(w).catch((e) => e))
    }
    for (const d of domainsToAdd) {
      tasks.push(addAllow.mutateAsync({ domain: d }).catch((e) => e))
    }
    for (const d of domainsToRemove) {
      tasks.push(removeAllow.mutateAsync(d).catch((e) => e))
    }

    if (tasks.length === 0) return

    const results = await Promise.all(tasks)
    const errors = results.filter((r) => r instanceof Error) as Error[]
    if (errors.length === 0) {
      notifications.show({
        color: 'green',
        title: 'Configuración guardada',
        message: 'Los cambios de moderación automática se aplicaron.',
      })
      // Reset originals al estado actual para que dirty vuelva a false.
      setOriginalSettings(draftSettings)
      setOriginalBanned(draftBanned)
      setOriginalAllowlist(draftAllowlist)
      setPendingWords([])
      setPendingAllowlist([])
    } else {
      for (const e of errors) {
        notifications.show({
          color: 'red',
          title: 'Error al guardar',
          message: formatAutomationError(e),
        })
      }
    }
  }

  return (
    <Stack gap="md">
      <Anchor component={Link} to={`/groups/${groupId}`} size="sm">
        ← Volver al grupo
      </Anchor>

      <Title order={2}>Moderación automática</Title>
      <Text size="sm" c="dimmed">
        Configura qué reglas aplica el bot a los mensajes del grupo y cómo reacciona.
      </Text>

      {/* Sección 1: switch principal */}
      <Paper withBorder p="lg" radius="md">
        <Stack gap="sm">
          <Title order={4}>General</Title>
          <Switch
            label="Habilitar moderación automática"
            checked={draftSettings.enabled}
            onChange={(e) => updateDraft('enabled', e.currentTarget.checked)}
          />
          <Text size="xs" c="dimmed">
            Cuando está deshabilitado, el bot no evalúa ninguna regla sobre los mensajes.
          </Text>
        </Stack>
      </Paper>

      {/* Sección 2: toggles por regla */}
      <Paper withBorder p="lg" radius="md">
        <Stack gap="sm">
          <Title order={4}>Reglas</Title>
          <Switch
            label="Flood (varios mensajes en pocos segundos)"
            checked={draftSettings.flood_enabled}
            onChange={(e) => updateDraft('flood_enabled', e.currentTarget.checked)}
          />
          <Switch
            label="Anti-spam (MAYÚSCULAS, caracteres repetidos, URLs muy cortas)"
            checked={draftSettings.anti_spam_enabled}
            onChange={(e) => updateDraft('anti_spam_enabled', e.currentTarget.checked)}
          />
          <Switch
            label="Anti-link (bloquea enlaces a dominios fuera de la allowlist)"
            checked={draftSettings.anti_link_enabled}
            onChange={(e) => updateDraft('anti_link_enabled', e.currentTarget.checked)}
          />
          <Switch
            label="Palabras prohibidas (lista de banned_words)"
            checked={draftSettings.banned_words_enabled}
            onChange={(e) => updateDraft('banned_words_enabled', e.currentTarget.checked)}
          />
        </Stack>
      </Paper>

      {/* Sección 3: thresholds */}
      <Paper withBorder p="lg" radius="md">
        <Stack gap="sm">
          <Title order={4}>Umbrales</Title>
          <Group grow align="flex-start">
            <NumberInput
              label="Flood: mensajes"
              value={draftSettings.flood_messages}
              min={1}
              onChange={(v) => updateDraft('flood_messages', typeof v === 'number' ? v : Number(v) || 1)}
            />
            <NumberInput
              label="Flood: segundos"
              value={draftSettings.flood_seconds}
              min={1}
              onChange={(v) => updateDraft('flood_seconds', typeof v === 'number' ? v : Number(v) || 1)}
            />
          </Group>
          <Group grow align="flex-start">
            <NumberInput
              label="Límite de advertencias"
              value={draftSettings.warning_limit}
              min={1}
              onChange={(v) => updateDraft('warning_limit', typeof v === 'number' ? v : Number(v) || 1)}
            />
            <NumberInput
              label="Días para expirar advertencias"
              value={draftSettings.warning_expire_days}
              min={1}
              onChange={(v) => updateDraft('warning_expire_days', typeof v === 'number' ? v : Number(v) || 1)}
            />
          </Group>
          <Group grow align="flex-start">
            <NumberInput
              label="Advertencias para auto-mute"
              value={draftSettings.automute_warnings}
              min={1}
              onChange={(v) => updateDraft('automute_warnings', typeof v === 'number' ? v : Number(v) || 1)}
            />
            <NumberInput
              label="Minutos del auto-mute"
              value={draftSettings.automute_minutes}
              min={1}
              onChange={(v) => updateDraft('automute_minutes', typeof v === 'number' ? v : Number(v) || 1)}
            />
            <NumberInput
              label="Advertencias para auto-ban"
              value={draftSettings.autoban_warnings}
              min={1}
              onChange={(v) => updateDraft('autoban_warnings', typeof v === 'number' ? v : Number(v) || 1)}
            />
          </Group>
        </Stack>
      </Paper>

      {/* Sección 4: listas */}
      <Paper withBorder p="lg" radius="md">
        <Stack gap="sm">
          <Title order={4}>Palabras prohibidas</Title>
          <Text size="xs" c="dimmed">
            Si el mensaje contiene alguna palabra (sin distinguir mayúsculas), suma una advertencia al autor.
          </Text>
          <TagsInput
            value={draftBanned}
            onChange={setDraftBanned}
            placeholder="Escribí una palabra y presioná Enter"
            splitChars={[',']}
            clearable
            data-testid="banned-words-input"
          />
          {/* Lista visible de pendientes (lo que se va a mandar al Save). */}
          {pendingWords.length > 0 ? (
            <Text size="xs" c="dimmed">
              Nuevas al guardar: {pendingWords.join(', ')}
            </Text>
          ) : null}
          <Group gap="xs">
            <Button variant="default" size="xs" onClick={() => setDraftBanned([])}>
              Vaciar
            </Button>
            <Button
              variant="default"
              size="xs"
              onClick={() => pendingWords.forEach((w) => handleAddWord(w))}
            >
              Marcar para agregar
            </Button>
          </Group>
        </Stack>
      </Paper>

      <Paper withBorder p="lg" radius="md">
        <Stack gap="sm">
          <Title order={4}>Allowlist de enlaces</Title>
          <Text size="xs" c="dimmed">
            Dominios autorizados. Anti-link solo bloquea enlaces a dominios NO listados acá (incluye subdominios: <code>example.com</code> cubre <code>sub.example.com</code>).
          </Text>
          <TagsInput
            value={draftAllowlist}
            onChange={setDraftAllowlist}
            placeholder="ejemplo.com"
            splitChars={[',']}
            clearable
            data-testid="link-allowlist-input"
          />
          {pendingAllowlist.length > 0 ? (
            <Text size="xs" c="dimmed">
              Nuevos al guardar: {pendingAllowlist.join(', ')}
            </Text>
          ) : null}
          <Group gap="xs">
            <Button variant="default" size="xs" onClick={() => setDraftAllowlist([])}>
              Vaciar
            </Button>
            <Button
              variant="default"
              size="xs"
              onClick={() => pendingAllowlist.forEach((d) => handleAddDomain(d))}
            >
              Marcar para agregar
            </Button>
          </Group>
        </Stack>
      </Paper>

      {/* Sección 5 (slice 2.1): warning visual al usuario */}
      <Paper withBorder p="lg" radius="md">
        <Stack gap="sm">
          <Title order={4}>Warning al usuario</Title>
          <Switch
            label="Avisar al usuario antes de silenciar/expulsar"
            checked={draftSettings.warn_user_enabled}
            onChange={(e) => updateDraft('warn_user_enabled', e.currentTarget.checked)}
            data-testid="warn-user-enabled-switch"
          />
          <Textarea
            label="Plantilla del warning (opcional)"
            description={
              draftSettings.warn_user_template
                ? 'Personalizada — vacío para volver al default.'
                : 'Vacío = usar la plantilla por defecto del backend.'
            }
            placeholder={DEFAULT_PRE_MUTE_PLACEHOLDER}
            autosize
            minRows={2}
            maxRows={5}
            maxLength={1000}
            value={draftSettings.warn_user_template ?? ''}
            onChange={(e) => {
              const next = e.currentTarget.value
              // empty string -> null (backend cae al default)
              updateDraft('warn_user_template', next === '' ? null : next)
            }}
            data-testid="warn-user-template-textarea"
          />
          <Text size="xs" c="dimmed">
            Variables disponibles: <code>{'{nombre}'}</code>, <code>{'{count}'}</code>,{' '}
            <code>{'{mute_minutes}'}</code>. Si no customizás la plantilla, el bot usa el
            default en español Rioplatense.
          </Text>
        </Stack>
      </Paper>

      <Group justify="flex-end">
        <Button
          variant="default"
          onClick={() => {
            if (originalSettings) setDraftSettings(originalSettings)
            setDraftBanned(originalBanned)
            setDraftAllowlist(originalAllowlist)
            setPendingWords([])
            setPendingAllowlist([])
          }}
          disabled={!dirty || isSaving}
        >
          Descartar cambios
        </Button>
        <Button onClick={handleSave} disabled={!dirty || isSaving} loading={isSaving}>
          Guardar
        </Button>
      </Group>
    </Stack>
  )
}
