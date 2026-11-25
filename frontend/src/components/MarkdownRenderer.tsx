import Box from '@mui/material/Box';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { markdownContentSx } from './markdownStyles';

// react-markdown never renders raw HTML and drops unsafe URL schemes
// (javascript: etc.), so user-written descriptions are safe to display.
export default function MarkdownRenderer({ children }: { children: string }) {
  return (
    <Box sx={markdownContentSx}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ node: _node, ...props }) => <a {...props} target="_blank" rel="noopener noreferrer" />,
        }}
      >
        {children}
      </ReactMarkdown>
    </Box>
  );
}
