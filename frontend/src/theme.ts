import { createTheme } from '@mui/material/styles';
import { LinkBehavior } from './components/LinkBehavior';

export const theme = createTheme({
  // Follows the operating system's light / dark preference.
  cssVariables: true,
  colorSchemes: {
    light: { palette: { primary: { main: '#2f5fd0' } } },
    dark: { palette: { primary: { main: '#6d93ff' } } },
  },
  shape: { borderRadius: 6 },
  components: {
    MuiLink: { defaultProps: { component: LinkBehavior } },
    MuiButtonBase: { defaultProps: { LinkComponent: LinkBehavior } },
    MuiButton: { defaultProps: { disableElevation: true }, styleOverrides: { root: { textTransform: 'none' } } },
    MuiTab: { styleOverrides: { root: { textTransform: 'none' } } },
    MuiToggleButton: { styleOverrides: { root: { textTransform: 'none' } } },
  },
});
