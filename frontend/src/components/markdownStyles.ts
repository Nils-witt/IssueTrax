import type { Theme } from '@mui/material/styles';
import type { SystemStyleObject } from '@mui/system';

/** Typography for rendered Markdown, shared by the viewer and the editor. */
export const markdownContentSx: SystemStyleObject<Theme> = {
  overflowWrap: 'anywhere',
  typography: 'body1',
  '& > :first-child': { mt: 0 },
  '& a': { color: 'primary.main' },
  '& img': { maxWidth: '100%' },
  '& code': { fontFamily: 'monospace', fontSize: '0.9em' },
  '& pre': { overflowX: 'auto', p: 1.5, bgcolor: 'action.hover', borderRadius: 1 },
  '& table': { borderCollapse: 'collapse' },
  '& th, & td': { border: 1, borderColor: 'divider', px: 1, py: 0.5 },
  '& blockquote': { m: 0, pl: 2, borderLeft: 3, borderColor: 'divider', color: 'text.secondary' },
};
