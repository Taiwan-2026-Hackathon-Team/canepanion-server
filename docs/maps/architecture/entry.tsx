import { createRoot } from 'react-dom/client'
import ArchitectureMap from './components/ArchitectureMap'
import { ARCHITECTURE } from './graph'

// Inline keyframes injected by esbuild css loader
import keyframes from './components/keyframes.css'

const style = document.createElement('style')
style.textContent = keyframes
document.head.appendChild(style)

const root = document.getElementById('root')
if (!root) throw new Error('missing #root')
createRoot(root).render(<ArchitectureMap data={ARCHITECTURE} />)
