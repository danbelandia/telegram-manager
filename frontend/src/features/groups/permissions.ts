// Traduccion es-AR de los permisos del bot (AGENTS 6). El backend expone
// las claves can_* en ingles; el panel las muestra como informacion, no
// como acciones. Las claves desconocidas se muestran tal cual llegan,
// nunca se eliminan silenciosamente.
const PERMISSION_LABELS: Record<string, string> = {
  can_send_messages: 'Enviar mensajes',
  can_send_audios: 'Enviar audios',
  can_send_documents: 'Enviar documentos',
  can_send_photos: 'Enviar fotos',
  can_send_videos: 'Enviar videos',
  can_send_video_notes: 'Enviar videonotas',
  can_send_voice_notes: 'Enviar notas de voz',
  can_send_polls: 'Enviar encuestas',
  can_send_other_messages: 'Enviar otros mensajes',
  can_add_web_page_previews: 'Agregar vistas previas de enlaces',
  can_change_info: 'Cambiar la información del grupo',
  can_invite_users: 'Invitar usuarios',
  can_pin_messages: 'Fijar mensajes',
  can_manage_topics: 'Gestionar temas',
  can_manage_video_chats: 'Gestionar videollamadas',
  can_post_messages: 'Publicar mensajes',
  can_edit_messages: 'Editar mensajes',
  can_delete_messages: 'Eliminar mensajes',
  can_restrict_members: 'Restringir miembros',
  can_promote_members: 'Ascender miembros',
  can_manage_chat: 'Gestionar el grupo',
}

/**
 * Devuelve la lista legible de permisos activos, en orden de llegada.
 * Permite distinguir en el panel que son capacidades del bot, no botones.
 */
export function formatPermissions(
  permissions: Record<string, boolean> | null | undefined,
): string[] {
  if (!permissions) return []
  return Object.entries(permissions)
    .filter(([, value]) => value)
    .map(([key]) => PERMISSION_LABELS[key] ?? key)
}