import Box from '@mui/material/Box';
import FormHelperText from '@mui/material/FormHelperText';
import FormLabel from '@mui/material/FormLabel';
import TextField from '@mui/material/TextField';
import { lazy, Suspense, useState } from 'react';
import { useTranslation } from 'react-i18next';

// MDXEditor (Lexical, CodeMirror) is large, so it is only loaded on pages
// that edit Markdown.
const RichMarkdownEditor = lazy(() => import('./RichMarkdownEditor'));

interface MarkdownEditorProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
}

/**
 * A WYSIWYG Markdown editor with toolbar and a source-mode toggle. Falls back
 * to a plain text field while loading and for Markdown the rich editor
 * cannot represent, so no content is ever lost.
 */
export function MarkdownEditor({ id, label, value, onChange, error }: MarkdownEditorProps) {
  const { t } = useTranslation();
  const [plain, setPlain] = useState(false);
  const hint = error ?? t('editor.markdownHint');

  const textField = (
    <TextField
      id={id}
      label={label}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      multiline
      minRows={10}
      fullWidth
      error={!!error}
      helperText={plain ? t('editor.plainFallback') : hint}
      slotProps={{ htmlInput: { style: { fontFamily: 'monospace', fontSize: '0.9rem' } } }}
    />
  );

  if (plain) return textField;

  return (
    <Suspense fallback={textField}>
      <Box id={id} role="group" aria-labelledby={`${id}-label`}>
        <FormLabel id={`${id}-label`} error={!!error} sx={{ display: 'block', mb: 0.75, typography: 'body2' }}>
          {label}
        </FormLabel>
        <RichMarkdownEditor label={label} value={value} onChange={onChange} onError={() => setPlain(true)} error={!!error} />
        <FormHelperText error={!!error} sx={{ mx: 1.75 }}>
          {hint}
        </FormHelperText>
      </Box>
    </Suspense>
  );
}
