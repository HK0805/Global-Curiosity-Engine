/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#060816",
        abyss: "#0b1226",
        cyan: "#38bdf8",
        violet: "#8b5cf6",
        amber: "#f59e0b",
        card: "rgba(10, 17, 35, 0.72)"
      },
      boxShadow: {
        glow: "0 0 0 1px rgba(56, 189, 248, 0.12), 0 20px 60px rgba(12, 18, 38, 0.55)",
        spike: "0 0 0 1px rgba(245, 158, 11, 0.18), 0 18px 45px rgba(245, 158, 11, 0.18)",
        panel: "inset 0 1px 0 rgba(255,255,255,0.04), 0 24px 70px rgba(2, 6, 18, 0.58)"
      },
      backgroundImage: {
        radar:
          "radial-gradient(circle at 20% 20%, rgba(56, 189, 248, 0.18), transparent 34%), radial-gradient(circle at 80% 15%, rgba(139, 92, 246, 0.14), transparent 28%), linear-gradient(180deg, #030610 0%, #070d1d 42%, #040710 100%)",
        panel:
          "linear-gradient(180deg, rgba(255,255,255,0.05), rgba(255,255,255,0) 18%), linear-gradient(180deg, rgba(4,9,21,0.92), rgba(7,11,24,0.88))"
      },
      fontFamily: {
        display: ["'Space Grotesk'", "sans-serif"],
        body: ["'Manrope'", "sans-serif"]
      },
      keyframes: {
        pulseGlow: {
          "0%, 100%": { opacity: "0.35", transform: "scale(1)" },
          "50%": { opacity: "0.9", transform: "scale(1.16)" }
        },
        drift: {
          "0%, 100%": { transform: "translateY(0px)" },
          "50%": { transform: "translateY(-6px)" }
        },
        sheen: {
          "0%": { transform: "translateX(-120%)" },
          "100%": { transform: "translateX(160%)" }
        }
      },
      animation: {
        "pulse-glow": "pulseGlow 2.6s ease-in-out infinite",
        drift: "drift 6.5s ease-in-out infinite",
        sheen: "sheen 1.8s ease-in-out infinite"
      }
    },
  },
  plugins: [],
};
