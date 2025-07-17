# Tavily-Load

![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

A high-performance proxy server for Tavily API with intelligent multi-key rotation, load balancing, and a modern web management interface.

## Features

- **🔄 Multi-key Rotation**: Automatic API key rotation with intelligent load balancing strategies
- **📊 Usage-Aware Selection**: Smart key selection based on remaining credits and usage patterns
- **💰 Cost Optimization**: Intelligent routing between plan credits and pay-as-you-go usage
- **🎯 Complete API Coverage**: Supports all Tavily API endpoints (Search, Extract, Crawl, Map, Usage)
- **� Modern Web Dashboard**: Beautiful, responsive management interface built with Next.js and shadcn/ui
- **� Real-time Analytics**: Live monitoring, performance metrics, and usage insights
- **🛡️ Intelligent Error Handling**: Smart blacklisting and retry mechanisms
- **⚡ High Performance**: Concurrent processing, connection pooling, and zero-copy streaming
- **🔧 Production Ready**: Graceful shutdown, comprehensive logging, and flexible configuration

## Quick Start

### Prerequisites
- Go 1.21+ (for building from source)
- Docker (for containerized deployment)

### Using Docker (Recommended)

```bash
# Clone and setup
git clone https://github.com/dbccccccc/tavily-load.git
cd tavily-load

# Add your Tavily API keys (one per line)
echo "tvly-your-api-key-1" > keys.txt
echo "tvly-your-api-key-2" >> keys.txt

# Run with Docker Compose
docker-compose up -d
```

### Build from Source

```bash
# Clone and build
git clone https://github.com/dbccccccc/tavily-load.git
cd tavily-load

# Setup configuration
make setup  # Creates .env and keys.txt from examples

# Build and run
make build
make run
```

**Access the service:**
- 🌐 **Web Dashboard**: http://localhost:3000
- 🔌 **API Endpoints**: http://localhost:3000/search, /extract, /health, etc.

## API Endpoints

### Tavily API (Proxy)
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/search` | POST | Tavily Search API |
| `/extract` | POST | Tavily Extract API |
| `/crawl` | POST | Tavily Crawl API (BETA) |
| `/map` | POST | Tavily Map API (BETA) |
| `/usage` | GET | Tavily Usage API |

### Management API
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check and system status |
| `/stats` | GET | Detailed statistics and key metrics |
| `/blacklist` | GET | View blacklisted keys |
| `/reset-keys` | GET | Reset all key states |
| `/usage-analytics` | GET | Comprehensive usage analytics |
| `/update-usage` | POST | Update usage from Tavily API |
| `/strategy` | GET/POST | Get or set selection strategy |

> **Note**: All endpoints are also available with `/api` prefix for frontend integration.

## Configuration

Create your configuration from the example:

```bash
cp .env.example .env
```

### Key Configuration Options

| Setting | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| Server Port | `PORT` | 3000 | Server listening port |
| Keys File | `KEYS_FILE` | keys.txt | API keys file path |
| Max Retries | `MAX_RETRIES` | 3 | Maximum retry attempts |
| Blacklist Threshold | `BLACKLIST_THRESHOLD` | 1 | Error count before blacklisting |
| Max Concurrent | `MAX_CONCURRENT_REQUESTS` | 100 | Maximum concurrent requests |
| Auth Key | `AUTH_KEY` | - | Optional authentication key |
| Log Level | `LOG_LEVEL` | info | Logging level (debug, info, warn, error) |
| Usage Tracking | `ENABLE_USAGE_TRACKING` | true | Enable intelligent usage tracking |
| Default Strategy | `DEFAULT_SELECTION_STRATEGY` | round_robin | Key selection strategy |

See `.env.example` for complete configuration options.

## Key Selection Strategies

| Strategy | Description | Best For |
|----------|-------------|----------|
| `round_robin` | **Default.** Round-robin selection across all available keys | Balanced usage across all keys |
| `plan_first` | Prefer plan credits over pay-as-you-go usage | Cost optimization when you have plan credits |

## Usage Examples

### Basic API Usage

```bash
# Search request
curl -X POST http://localhost:3000/search \
  -H "Content-Type: application/json" \
  -d '{"query": "What is artificial intelligence?", "max_results": 5}'

# Extract content
curl -X POST http://localhost:3000/extract \
  -H "Content-Type: application/json" \
  -d '{"urls": ["https://example.com/article"]}'
```

### Management

```bash
# Health check
curl http://localhost:3000/health

# View statistics
curl http://localhost:3000/stats

# Set strategy
curl -X POST http://localhost:3000/strategy \
  -H "Content-Type: application/json" \
  -d '{"strategy": "plan_first"}'
```

## Development

### Available Commands

```bash
# Setup and Build
make setup      # Setup development environment
make build      # Build binary and frontend
make clean      # Clean build files

# Run and Test
make run        # Run server
make dev        # Development mode with race detection
make test       # Run tests
make coverage   # Generate coverage report

# Code Quality
make lint       # Code linting
make fmt        # Format code

# Management
make health     # Health check
make stats      # View statistics

# Help
make help       # Show all commands
```

### Project Structure

```text
tavily-load/
├── cmd/tavily-load/        # Main application entry point
├── internal/               # Private application code
│   ├── config/            # Configuration management
│   ├── handler/           # HTTP handlers
│   ├── keymanager/        # API key management
│   ├── proxy/             # Proxy server core
│   └── usage/             # Usage tracking
├── web/                   # Frontend (Next.js)
├── pkg/types/             # Shared types and interfaces
├── .env.example           # Configuration template
├── keys.txt.example       # API keys template
├── Dockerfile             # Container build
├── docker-compose.yml     # Multi-container setup
└── Makefile              # Build automation
```

## Troubleshooting

### Common Issues

**Frontend not loading:**
- Verify `/health` endpoint works
- Check `docker logs container-name` for errors
- Ensure frontend was built (look for `web/out` directory)

**API key issues:**
```bash
# Check key status
curl http://localhost:3000/stats

# Reset if needed
curl http://localhost:3000/reset-keys
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
