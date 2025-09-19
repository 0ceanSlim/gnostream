/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./www/views/**/*.html",
    "./www/res/**/*.{html,js}",
    "../views/**/*.html",
    "../res/**/*.{html,js}",
  ],
  theme: {
    extend: {
      colors: {
        'cyber-green': '#00ff41',
        'cyber-blue': '#0ff', 
        'cyber-pink': '#ff0080',
        'cyber-yellow': '#ffff00',
        'bg-matrix': '#0d0d0d',
        'bg-terminal': '#001100',
        'text-matrix': '#00ff41',
        'text-glow': '#00ff4180',
      },
      fontFamily: {
        'mono': ['Share Tech Mono', 'monospace'],
        'cyber': ['Orbitron', 'monospace'],
      },
    },
  },
  plugins: [],
}