# WORK IN PROGRESS, NOT ALL OF THIS IS RIGHT BUT THIS PROBABLY WORKS AS IS IF YOU BUILD IT AND SET THE CONFIGS

# [STREAM_NODE] - Cyberpunk Live Stream Server 🤖⚡

A Matrix-inspired, cyberpunk-themed live streaming server built with Go and modern web technologies. Features a terminal-style interface with glitch effects, neon accents, and that authentic hacker aesthetic.

![Cyberpunk Theme](https://img.shields.io/badge/Theme-Cyberpunk-00ff41?style=flat-square)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)
![HTML5](https://img.shields.io/badge/HTML5-E34F26?style=flat-square&logo=html5&logoColor=white)
![TailwindCSS](https://img.shields.io/badge/Tailwind-38B2AC?style=flat-square&logo=tailwind-css&logoColor=white)

## ✨ Features

### 🎬 Core Streaming

- **Live HLS streaming** with auto-refresh and status monitoring
- **Stream archive system** with metadata and playback
- **Real-time stream status** updates every 10 seconds
- **Video controls** with fullscreen support
- **Mobile responsive** interface

### 🌆 Cyberpunk UI/UX

- **Matrix rain effect** with falling characters
- **Terminal-style interface** with scan lines and glitch effects
- **Neon color scheme** (green/cyan/pink) with subtle glow effects
- **Custom cyber buttons** with hover animations
- **Glitch text effects** that trigger randomly
- **Monospace fonts** (Share Tech Mono, Orbitron)
- **Grid background pattern** for that authentic cyber feel

### 🔧 Technical Stack

- **Backend**: Go with custom web server and HLS support
- **Frontend**: Vanilla JS with HTMX for seamless navigation
- **Styling**: TailwindCSS with custom cyberpunk theme
- **Video**: HLS.js for cross-browser compatibility
- **Architecture**: Template-based rendering with Go templates

## 🚀 Quick Start

### Prerequisites

- Go 1.21 or higher
- FFmpeg (for stream processing)
- Modern web browser with HLS support

### Installation

1. **Clone the repository**

   ```bash
   git clone <repository-url>
   cd gnostream
   ```

2. **Configure the server**

   ```bash
   cp configs/config.example.yml configs/config.yml
   # Edit configs/config.yml with your settings
   ```

3. **Build and run**

   ```bash
   go build -o stream-server
   ./stream-server
   ```

4. **Access the interface**
   ```
   Open http://localhost:8080 in your browser
   ```

## ⚙️ Configuration

Edit `configs/config.yml` to customize:

```yaml
server:
  port: 8080
  host: localhost

stream:
  output_dir: "./streams/live"
  archive_dir: "./streams/archive"
# Add other configuration options as needed
```

## 📁 Project Structure

```
gnostream/
├── configs/
│   └── config.yml                 # Server configuration
├── internal/
│   ├── config/                    # Configuration handling
│   ├── stream/                    # Stream monitoring logic
│   └── web/                       # Web server and routing
├── www/                           # Frontend assets
│   ├── views/
│   │   ├── templates/             # Go template layouts
│   │   │   ├── layout.html        # Main cyberpunk layout
│   │   │   ├── header.html        # Terminal-style header
│   │   │   └── footer.html        # System info footer
│   │   ├── components/            # Reusable UI components
│   │   │   ├── video-player.html  # HLS video player
│   │   │   ├── stream-info.html   # Metadata display
│   │   │   └── navigation.html    # Cyber navigation
│   │   ├── live.html              # Live stream page
│   │   └── archive.html           # Stream archive page
│   └── res/
│       ├── js/
│       │   ├── stream.js          # Live streaming logic
│       │   └── archive.js         # Archive browsing
│       └── style/
│           └── input.css          # Custom cyberpunk styles
└── README.md
```

## 🎮 Usage

### Live Streaming

1. Start your stream (via OBS, FFmpeg, etc.) to the configured endpoint
2. Navigate to the **LIVE_FEED** section
3. The interface will automatically detect when the stream goes live
4. Stream status updates in real-time with neon indicators

### Archive Access

1. Go to **DATA_VAULT** section
2. Browse previous streams with cyberpunk-styled cards
3. Click any stream to open the neural viewer modal
4. Full metadata and playback controls available

### API Endpoints

- `GET /api/stream-data` - Current stream metadata as JSON
- `GET /api/health` - Server health check
- `GET /live/` - HLS stream files (with CORS)
- `GET /past-streams/` - Archive directory access

## 🎨 Customization

### Color Scheme

Edit CSS variables in `layout.html`:

```css
:root {
  --cyber-green: #00ff41; /* Matrix green */
  --cyber-blue: #0ff; /* Cyan accents */
  --cyber-pink: #ff0080; /* Live status */
  --bg-matrix: #0d0d0d; /* Background */
  --bg-terminal: #001100; /* Terminal boxes */
}
```

### Fonts

Current cyberpunk fonts:

- **Share Tech Mono**: Terminal/code text
- **Orbitron**: Headers and titles

### Effects

- Matrix rain: Configurable in `layout.html` JavaScript
- Glitch effects: Auto-trigger every 3 seconds on random elements
- Scan lines: CSS animations on terminal boxes
- Neon glow: Two intensity levels (subtle/normal)

## 🔧 Development

### Adding New Components

1. Create HTML template in `www/views/components/`
2. Use cyberpunk classes: `terminal-box`, `cyber-button`, `neon-glow-subtle`
3. Add terminal headers with file names (e.g., `COMPONENT_NAME.exe`)
4. Include status indicators: `●`, `◉`, `▣`

### Styling Guidelines

- Use `font-mono` for all text
- Terminal-style naming: `NEURAL_DATA.stream`, `ACCESS_LEVEL.exe`
- Color hierarchy: Green (primary) → Cyan (secondary) → Pink (alerts)
- Always include hover effects and transitions

### JavaScript Patterns

```javascript
// Cyberpunk card generation
function createCyberCard(data) {
  return `
        <div class="p-4 transition-all transform rounded-md cursor-pointer terminal-box hover:scale-105 hover:shadow-lg hover:shadow-cyan-500/20">
            <div class="mb-2 font-mono text-xs text-cyan-400">
                NEURAL_${data.id}.stream
                <span class="ml-auto text-green-400">●</span>
            </div>
            <!-- Content -->
        </div>
    `;
}
```

## 🐛 Troubleshooting

### Stream Not Showing

1. Check stream endpoint configuration
2. Verify FFmpeg output format (HLS recommended)
3. Check browser console for HLS.js errors
4. Ensure CORS headers are set for cross-origin requests

### Styling Issues

1. Clear browser cache for CSS updates
2. Check TailwindCSS CDN connection
3. Verify Google Fonts loading (Share Tech Mono, Orbitron)

### Performance

- Matrix rain effect can be disabled by commenting out `createMatrixRain()`
- Glitch effects can be reduced by increasing the interval in `layout.html`

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/neural-enhancement`
3. Follow the cyberpunk naming conventions
4. Test with multiple browsers
5. Submit a pull request

## 📜 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🙏 Credits

- **OceanSlim**: Original creator and neural architect
- **Matrix (1999)**: Visual inspiration
- **Cyberpunk 2077**: Color palette influence
- **Ghost in the Shell**: Terminal aesthetics

---

**STATUS: OPERATIONAL** | **NEURAL_INTERFACE: v2.1.4** | **ACCESS_LEVEL: AUTHORIZED**

> "Welcome to the neural stream matrix. Jack in and experience consciousness through code."
