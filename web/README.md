# Tavily Load Balancer Frontend

Modern web interface for managing the Tavily API load balancer built with Next.js and shadcn/ui.

## Features

- **Dashboard Overview** - Real-time stats and system health monitoring
- **API Key Management** - Add, edit, and monitor API keys with detailed metrics
- **Usage Analytics** - Comprehensive usage tracking and performance analytics
- **Strategy Configuration** - Configure load balancing strategies and error handling
- **Responsive Design** - Works on desktop, tablet, and mobile devices
- **Dark/Light Mode** - Built-in theme support (coming soon)

## Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Start production server
npm start
```

## Integration

The frontend is automatically served by the Go backend when built. The backend serves:

- API endpoints at `/api/*` 
- Frontend static files at all other routes
- Legacy API endpoints (backward compatibility)

## Technology Stack

- **Next.js 15** - React framework with App Router
- **TypeScript** - Type safety and better developer experience  
- **Tailwind CSS** - Utility-first CSS framework
- **shadcn/ui** - High-quality, customizable UI components
- **Lucide React** - Beautiful, consistent icons
- **Recharts** - Composable charting library (for future analytics)