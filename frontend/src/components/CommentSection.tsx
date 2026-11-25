import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import IconButton from '@mui/material/IconButton';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlined';
import EditIcon from '@mui/icons-material/Edit';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useComments, useCreateComment, useDeleteComment, useMe, useUpdateComment } from '../api/hooks';
import type { Comment, Issue } from '../api/types';
import { fieldErrorText } from '../lib/errors';
import { DateTime } from './DateTime';
import { ErrorMessage } from './ErrorMessage';
import { Loading } from './Loading';
import { Markdown } from './Markdown';
import { MarkdownEditor } from './MarkdownEditor';

/** The issue's comments, oldest first, with a form to add one at the end. */
export function CommentSection({ issue }: { issue: Issue }) {
  const { t } = useTranslation();
  const comments = useComments(issue.projectKey, issue.number);
  const me = useMe();
  const archived = issue.status === 'archived';

  return (
    <Stack component="section" spacing={2} aria-labelledby="comments-title" sx={{ minWidth: 0 }}>
      <Typography id="comments-title" variant="overline" component="h3" color="text.secondary" sx={{ lineHeight: 2 }}>
        {comments.data ? t('comments.count', { count: comments.data.length }) : t('comments.title')}
      </Typography>

      <ErrorMessage error={comments.error} />
      {comments.isPending && <Loading />}
      {comments.data?.length === 0 && <Typography color="text.secondary">{t('comments.empty')}</Typography>}
      {comments.data?.map((c) => (
        <CommentItem key={c.id} issue={issue} comment={c} editable={!archived && c.author.id === me.data?.id} />
      ))}

      {archived ? (
        <Typography color="text.secondary">{t('comments.archived')}</Typography>
      ) : (
        <NewComment issue={issue} />
      )}
    </Stack>
  );
}

function CommentItem({ issue, comment, editable }: { issue: Issue; comment: Comment; editable: boolean }) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const remove = useDeleteComment(issue.projectKey, issue.number);
  const edited = comment.updatedAt !== comment.createdAt;

  return (
    <Paper variant="outlined" component="article" sx={{ minWidth: 0 }}>
      <Stack
        direction="row"
        spacing={1}
        sx={{ px: 2, py: 0.5, alignItems: 'center', bgcolor: 'action.hover', borderBottom: 1, borderColor: 'divider' }}
      >
        <Typography variant="body2" sx={{ flex: 1, minWidth: 0 }}>
          <Box component="strong" title={comment.author.email}>
            {comment.author.displayName || '?'}
          </Box>{' '}
          <Box component="span" sx={{ color: 'text.secondary' }}>
            <DateTime value={comment.createdAt} />
            {edited && (
              <>
                {' · '}
                <span title={new Date(comment.updatedAt).toISOString()}>{t('comments.edited')}</span>
              </>
            )}
          </Box>
        </Typography>
        {editable && !editing && (
          <>
            <IconButton size="small" aria-label={t('comments.edit')} title={t('comments.edit')} onClick={() => setEditing(true)}>
              <EditIcon fontSize="small" />
            </IconButton>
            <IconButton size="small" aria-label={t('comments.delete')} title={t('comments.delete')} onClick={() => setConfirming(true)}>
              <DeleteOutlineIcon fontSize="small" />
            </IconButton>
          </>
        )}
      </Stack>

      <Box sx={{ px: 2, py: 1.5 }}>
        {editing ? (
          <EditComment issue={issue} comment={comment} onDone={() => setEditing(false)} />
        ) : (
          <Markdown>{comment.body}</Markdown>
        )}
        <ErrorMessage error={remove.error} />
      </Box>

      <Dialog open={confirming} onClose={() => setConfirming(false)}>
        <DialogContent>
          <DialogContentText>{t('comments.deleteConfirm')}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirming(false)}>{t('app.cancel')}</Button>
          <Button
            color="error"
            variant="contained"
            loading={remove.isPending}
            onClick={() => remove.mutate(comment.id, { onSettled: () => setConfirming(false) })}
          >
            {t('app.delete')}
          </Button>
        </DialogActions>
      </Dialog>
    </Paper>
  );
}

function EditComment({ issue, comment, onDone }: { issue: Issue; comment: Comment; onDone: () => void }) {
  const { t } = useTranslation();
  const update = useUpdateComment(issue.projectKey, issue.number);
  const [body, setBody] = useState(comment.body);

  function submit(e: FormEvent) {
    e.preventDefault();
    if (body.trim() === comment.body) onDone();
    else update.mutate({ id: comment.id, body }, { onSuccess: onDone });
  }

  return (
    <Stack component="form" spacing={1.5} onSubmit={submit}>
      <MarkdownEditor
        id={`comment-${comment.id}`}
        label={t('comments.edit')}
        value={body}
        onChange={setBody}
        error={fieldErrorText(t, update.error, 'body')}
      />
      <ErrorMessage error={update.error} />
      <Stack direction="row" spacing={1}>
        <Button type="submit" variant="contained" loading={update.isPending} disabled={!body.trim()}>
          {t('app.save')}
        </Button>
        <Button onClick={onDone}>{t('app.cancel')}</Button>
      </Stack>
    </Stack>
  );
}

function NewComment({ issue }: { issue: Issue }) {
  const { t } = useTranslation();
  const create = useCreateComment(issue.projectKey, issue.number);
  const [body, setBody] = useState('');
  // Bumped after posting so the (uncontrolled) rich editor starts empty again.
  const [editorKey, setEditorKey] = useState(0);

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate(body, {
      onSuccess: () => {
        setBody('');
        setEditorKey((k) => k + 1);
      },
    });
  }

  return (
    <Stack component="form" spacing={1.5} onSubmit={submit}>
      <MarkdownEditor
        key={editorKey}
        id="new-comment"
        label={t('comments.new')}
        value={body}
        onChange={setBody}
        error={fieldErrorText(t, create.error, 'body')}
      />
      <ErrorMessage error={create.error} />
      <Box>
        <Button type="submit" variant="contained" loading={create.isPending} disabled={!body.trim()}>
          {t('comments.submit')}
        </Button>
      </Box>
    </Stack>
  );
}
