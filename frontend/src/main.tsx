import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import './index.css'
import './i18n'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { ConfirmationHost } from './components/ConfirmationHost'

createRoot(document.getElementById('root')!).render(<><App /><ConfirmationHost /></>)
