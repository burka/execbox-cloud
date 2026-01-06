#!/bin/bash
# Development Database Setup Helper
# This script helps set up the development environment

set -e

echo "🚀 Execbox Cloud Development Setup"
echo "=================================="
echo ""

# Check prerequisites
check_prereqs() {
    echo "🔍 Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        echo "❌ Docker is not installed. Please install Docker first."
        echo "   Visit: https://docs.docker.com/get-docker/"
        exit 1
    fi
    echo "✅ Docker is installed"
    
    if ! command -v docker-compose &> /dev/null; then
        echo "❌ Docker Compose is not installed. Please install Docker Compose first."
        echo "   Visit: https://docs.docker.com/compose/install/"
        exit 1
    fi
    echo "✅ Docker Compose is installed"
    
    if ! command -v go &> /dev/null; then
        echo "❌ Go is not installed. Please install Go first."
        echo "   Visit: https://golang.org/dl/"
        exit 1
    fi
    echo "✅ Go is installed"
    
    if ! command -v npm &> /dev/null; then
        echo "❌ npm is not installed. Please install Node.js and npm first."
        echo "   Visit: https://nodejs.org/"
        exit 1
    fi
    echo "✅ Node.js/npm is installed"
    
    echo ""
}

# Setup environment
setup_environment() {
    echo "📝 Setting up environment..."
    
    if [ ! -f .env ]; then
        make setup-env
        echo ""
        echo "⚠️  IMPORTANT: Please edit .env file and update:"
        echo "   • FLY_API_TOKEN - Your Fly.io API token"
        echo "   • SUPABASE_URL - If using Supabase instead of local PostgreSQL"
        echo "   • SUPABASE_ANON_KEY - If using Supabase"
        echo ""
        read -p "Press Enter after updating .env file..."
    else
        echo "✅ .env file already exists"
    fi
}

# Install dependencies
install_dependencies() {
    echo "📦 Installing dependencies..."
    
    echo "Installing Go modules..."
    go mod download
    go mod tidy
    
    echo "Installing frontend dependencies..."
    cd dashboard
    npm install
    cd ..
    
    echo "✅ Dependencies installed"
    echo ""
}

# Start database
start_database() {
    echo "🗄️  Starting development database on port 5433..."
    echo "   (Using port 5433 to avoid conflicts with local PostgreSQL instances)"
    make run-devdb
    echo ""
}

# Final setup
final_setup() {
    echo "🎯 Running final setup..."
    
    # Load environment and run migrations
    source .env
    make db-migrate
    
    echo ""
    echo "🎉 Development setup complete!"
    echo ""
    echo "📋 Next Steps:"
    echo "1. Update your .env file with real credentials"
    echo "2. Start the backend server:"
    echo "   make run"
    echo ""
    echo "3. In another terminal, start the frontend:"
    echo "   make dev-frontend"
    echo ""
    echo "4. Open http://localhost:5173 in your browser"
    echo ""
    echo "🔍 Useful commands:"
    echo "   • make db-shell     - Open PostgreSQL shell"
    echo "   • make db-logs      - View database logs"
    echo "   • make stop-devdb   - Stop database"
    echo "   • make check-env    - Check environment status"
    echo ""
}

# Main execution
main() {
    check_prereqs
    setup_environment
    install_dependencies
    start_database
    final_setup
}

# Run if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi