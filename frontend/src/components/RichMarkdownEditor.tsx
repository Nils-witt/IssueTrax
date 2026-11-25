import '@mdxeditor/editor/style.css';
import {
  BlockTypeSelect,
  BoldItalicUnderlineToggles,
  codeBlockPlugin,
  CodeMirrorEditor,
  codeMirrorPlugin,
  CodeToggle,
  ConditionalContents,
  CreateLink,
  diffSourcePlugin,
  DiffSourceToggleWrapper,
  headingsPlugin,
  imagePlugin,
  InsertCodeBlock,
  InsertTable,
  InsertThematicBreak,
  linkDialogPlugin,
  linkPlugin,
  listsPlugin,
  ListsToggle,
  markdownShortcutPlugin,
  MDXEditor,
  type MDXEditorMethods,
  quotePlugin,
  Separator,
  StrikeThroughSupSubToggles,
  tablePlugin,
  thematicBreakPlugin,
  toolbarPlugin,
  UndoRedo,
  type Translation,
} from '@mdxeditor/editor';
import Box from '@mui/material/Box';
import { useColorScheme } from '@mui/material/styles';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { mdxEditorMessages } from '../locales/mdxeditor';
import { markdownContentSx } from './markdownStyles';

export interface RichMarkdownEditorProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** Called when the Markdown cannot be loaded into the rich editor. */
  onError: () => void;
  error?: boolean;
}

const codeBlockLanguages = {
  '': 'Plain text',
  bash: 'Bash',
  css: 'CSS',
  go: 'Go',
  html: 'HTML',
  java: 'Java',
  js: 'JavaScript',
  json: 'JSON',
  jsx: 'JSX',
  py: 'Python',
  sql: 'SQL',
  ts: 'TypeScript',
  tsx: 'TSX',
  yaml: 'YAML',
};

// Only Markdown features that round-trip as plain Markdown are offered:
// underline, image sizes and similar would be written as raw HTML, which the
// viewer deliberately does not render.
const plugins = [
  headingsPlugin(),
  listsPlugin(),
  quotePlugin(),
  thematicBreakPlugin(),
  linkPlugin(),
  linkDialogPlugin(),
  imagePlugin({ disableImageResize: true, disableImageSettingsButton: true }),
  tablePlugin(),
  codeBlockPlugin({
    defaultCodeBlockLanguage: '',
    // Code blocks in a language not in the list are still editable.
    codeBlockEditorDescriptors: [{ priority: -10, match: () => true, Editor: CodeMirrorEditor }],
  }),
  codeMirrorPlugin({ codeBlockLanguages }),
  markdownShortcutPlugin(),
  diffSourcePlugin({ viewMode: 'rich-text' }),
  toolbarPlugin({
    toolbarContents: () => (
      <DiffSourceToggleWrapper options={['rich-text', 'source']}>
        <ConditionalContents
          options={[
            { when: (editor) => editor?.editorType === 'codeblock', contents: () => null },
            {
              fallback: () => (
                <>
                  <UndoRedo />
                  <Separator />
                  <BlockTypeSelect />
                  <BoldItalicUnderlineToggles options={['Bold', 'Italic']} />
                  <StrikeThroughSupSubToggles options={['Strikethrough']} />
                  <CodeToggle />
                  <Separator />
                  <ListsToggle />
                  <Separator />
                  <CreateLink />
                  <InsertTable />
                  <InsertCodeBlock />
                  <InsertThematicBreak />
                </>
              ),
            },
          ]}
        />
      </DiffSourceToggleWrapper>
    ),
  }),
];

/** WYSIWYG Markdown editor (MDXEditor) styled to fit the MUI theme. */
export default function RichMarkdownEditor({ label, value, onChange, onError, error }: RichMarkdownEditorProps) {
  const { i18n } = useTranslation();
  const { mode, systemMode } = useColorScheme();
  const dark = (mode === 'system' ? systemMode : mode) === 'dark';
  const ref = useRef<MDXEditorMethods>(null);
  // The editor only reads `markdown` on mount; remember what it last reported
  // so that value changes coming from outside can be pushed into it.
  const lastValue = useRef(value);
  const [initialMarkdown] = useState(value);

  useEffect(() => {
    if (ref.current && value !== lastValue.current) {
      lastValue.current = value;
      ref.current.setMarkdown(value);
    }
  }, [value]);

  const lng = i18n.resolvedLanguage ?? 'en';
  const translation = useMemo<Translation>(
    () => (key, defaultValue, interpolations) => {
      if (key === 'contentArea.editableMarkdown') return label;
      const template = mdxEditorMessages[lng]?.[key] ?? defaultValue;
      return template.replace(/\{\{(\w+)\}\}/g, (_, name: string) => String(interpolations?.[name] ?? ''));
    },
    [lng, label],
  );

  return (
    <Box
      sx={{
        border: 1,
        borderColor: error ? 'error.main' : 'divider',
        borderRadius: 1,
        overflow: 'hidden',
        bgcolor: 'background.paper',
        '&:focus-within': {
          borderColor: error ? 'error.main' : 'primary.main',
          boxShadow: (t) => `0 0 0 1px ${error ? t.palette.error.main : t.palette.primary.main}`,
        },
        '& .mdxeditor': { fontFamily: 'inherit', '--baseBg': 'transparent', '--basePageBg': 'transparent' },
        '& .mdxeditor-toolbar': { borderRadius: 0, borderBottom: 1, borderColor: 'divider' },
        '& .issuetrax-md-content': { ...markdownContentSx, minHeight: '10rem', px: 2, py: 1.5 },
        '& .cm-editor': { fontSize: '0.9rem' },
      }}
    >
      <MDXEditor
        ref={ref}
        markdown={initialMarkdown}
        onChange={(md, initialNormalize) => {
          lastValue.current = md;
          if (!initialNormalize) onChange(md);
        }}
        onError={onError}
        plugins={plugins}
        translation={translation}
        suppressHtmlProcessing
        className={dark ? 'dark-theme' : undefined}
        contentEditableClassName="issuetrax-md-content"
      />
    </Box>
  );
}
