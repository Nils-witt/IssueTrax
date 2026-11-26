import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import IconButton from '@mui/material/IconButton';
import Link from '@mui/material/Link';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import AttachFileIcon from '@mui/icons-material/AttachFile';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlined';
import InsertDriveFileOutlinedIcon from '@mui/icons-material/InsertDriveFileOutlined';
import { useRef, useState, type ChangeEvent, type DragEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/client';
import { attachmentUrl, useAttachments, useDeleteAttachment, useMe, useUploadAttachment } from '../api/hooks';
import { maxAttachmentBytes, type Attachment, type Issue } from '../api/types';
import { errorText } from '../lib/errors';
import { formatBytes } from '../lib/format';
import { DateTime } from './DateTime';
import { ErrorMessage } from './ErrorMessage';
import { Loading } from './Loading';

/** Image types the server shows inline, so they get a thumbnail. */
const previewTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/webp'];

const isPreviewable = (a: Attachment) => previewTypes.includes(a.mediaType.split(';')[0].trim());

interface Failure {
  name: string;
  message: string;
}

/** The issue's files, oldest first, with a button and drop zone to attach more. */
export function AttachmentSection({ issue }: { issue: Issue }) {
  const { t, i18n } = useTranslation();
  const attachments = useAttachments(issue.projectKey, issue.number);
  const upload = useUploadAttachment(issue.projectKey, issue.number);
  const me = useMe();
  const archived = issue.status === 'archived';
  const input = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState<string | null>(null);
  const [failures, setFailures] = useState<Failure[]>([]);
  const [dragging, setDragging] = useState(false);
  const max = formatBytes(maxAttachmentBytes, i18n.resolvedLanguage);

  function failureText(file: File, err: unknown): string {
    if (err instanceof ApiError && err.fields.file === 'too_long') return t('attachments.tooLarge', { name: file.name, max });
    if (err instanceof ApiError && err.fields.file === 'required') return t('attachments.emptyFile', { name: file.name });

    return t('attachments.failed', { name: file.name, error: errorText(t, err) });
  }

  /** Uploads files one after another, collecting what failed. */
  async function uploadFiles(files: File[]) {
    const failed: Failure[] = [];
    setFailures([]);

    for (const file of files) {
      // Checked here too, so a large file is not sent only to be refused.
      if (file.size > maxAttachmentBytes) {
        failed.push({ name: file.name, message: t('attachments.tooLarge', { name: file.name, max }) });
        continue;
      }

      setUploading(file.name);
      try {
        await upload.mutateAsync(file);
      } catch (err) {
        failed.push({ name: file.name, message: failureText(file, err) });
      }
    }

    setUploading(null);
    setFailures(failed);
  }

  function onPick(e: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? []);
    e.target.value = ''; // picking the same file again fires onChange again
    if (files.length > 0) void uploadFiles(files);
  }

  const canUpload = !archived && uploading === null;

  const dropHandlers = archived
    ? {}
    : {
        onDragOver: (e: DragEvent) => {
          if (!e.dataTransfer.types.includes('Files')) return;
          e.preventDefault();
          setDragging(true);
        },
        onDragLeave: (e: DragEvent) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false);
        },
        onDrop: (e: DragEvent) => {
          e.preventDefault();
          setDragging(false);
          const files = Array.from(e.dataTransfer.files);
          if (files.length > 0 && canUpload) void uploadFiles(files);
        },
      };

  return (
    <Stack component="section" spacing={1.5} aria-labelledby="attachments-title" sx={{ minWidth: 0 }} {...dropHandlers}>
      <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography id="attachments-title" variant="overline" component="h3" color="text.secondary" sx={{ lineHeight: 2 }}>
          {attachments.data ? t('attachments.count', { count: attachments.data.length }) : t('attachments.title')}
        </Typography>
        {!archived && (
          <>
            <input ref={input} type="file" multiple hidden onChange={onPick} aria-hidden tabIndex={-1} />
            <Button size="small" startIcon={<AttachFileIcon />} onClick={() => input.current?.click()} loading={uploading !== null} disabled={!canUpload}>
              {t('attachments.add')}
            </Button>
          </>
        )}
      </Stack>

      <ErrorMessage error={attachments.error} />
      {attachments.isPending && <Loading />}

      <Paper
        variant="outlined"
        sx={{
          minWidth: 0,
          borderStyle: dragging ? 'dashed' : 'solid',
          borderColor: dragging ? 'primary.main' : 'divider',
          bgcolor: dragging ? 'action.hover' : undefined,
        }}
      >
        {attachments.data?.length === 0 && (
          <Typography color="text.secondary" sx={{ px: 2, py: 1.5 }}>
            {archived ? t('attachments.empty') : t('attachments.emptyDrop')}
          </Typography>
        )}
        {attachments.data?.map((a, i) => (
          <AttachmentItem key={a.id} issue={issue} attachment={a} divider={i > 0} deletable={!archived && a.uploader.id === me.data?.id} />
        ))}
        {uploading !== null && (
          <Typography variant="body2" color="text.secondary" role="status" sx={{ px: 2, py: 1, borderTop: 1, borderColor: 'divider' }}>
            {t('attachments.uploading', { name: uploading })}
          </Typography>
        )}
      </Paper>

      {failures.map((f, i) => (
        <Typography key={`${i}-${f.name}`} variant="body2" color="error" role="alert">
          {f.message}
        </Typography>
      ))}
      {archived && (
        <Typography variant="body2" color="text.secondary">
          {t('attachments.archived')}
        </Typography>
      )}
    </Stack>
  );
}

function AttachmentItem({ issue, attachment, divider, deletable }: { issue: Issue; attachment: Attachment; divider: boolean; deletable: boolean }) {
  const { t, i18n } = useTranslation();
  const [confirming, setConfirming] = useState(false);
  const remove = useDeleteAttachment(issue.projectKey, issue.number);
  const url = attachmentUrl(issue.projectKey, issue.number, attachment.id);
  const preview = isPreviewable(attachment);

  return (
    <Box sx={{ px: 2, py: 1, borderTop: divider ? 1 : 0, borderColor: 'divider' }}>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
        {preview ? (
          <Box
            component="img"
            src={url}
            alt=""
            loading="lazy"
            sx={{ width: 40, height: 40, objectFit: 'cover', borderRadius: 1, flexShrink: 0, bgcolor: 'action.hover' }}
          />
        ) : (
          <InsertDriveFileOutlinedIcon color="action" sx={{ width: 40, flexShrink: 0 }} />
        )}

        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Link
            href={url}
            // Images open in a tab; everything else is served as a download.
            {...(preview ? { target: '_blank', rel: 'noopener' } : {})}
            sx={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
            title={attachment.fileName}
          >
            {attachment.fileName}
          </Link>
          <Typography variant="body2" color="text.secondary">
            {formatBytes(attachment.size, i18n.resolvedLanguage)} · <span title={attachment.uploader.email}>{attachment.uploader.displayName || '?'}</span> ·{' '}
            <DateTime value={attachment.createdAt} />
          </Typography>
        </Box>

        {deletable && (
          <IconButton size="small" aria-label={t('attachments.delete')} title={t('attachments.delete')} onClick={() => setConfirming(true)}>
            <DeleteOutlineIcon fontSize="small" />
          </IconButton>
        )}
      </Stack>

      <ErrorMessage error={remove.error} />

      <Dialog open={confirming} onClose={() => setConfirming(false)}>
        <DialogContent>
          <DialogContentText>{t('attachments.deleteConfirm', { name: attachment.fileName })}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirming(false)}>{t('app.cancel')}</Button>
          <Button
            color="error"
            variant="contained"
            loading={remove.isPending}
            onClick={() => remove.mutate(attachment.id, { onSettled: () => setConfirming(false) })}
          >
            {t('app.delete')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
