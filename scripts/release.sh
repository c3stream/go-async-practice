#!/bin/bash

# Release Management Script for Go Async Practice
# Usage: ./scripts/release.sh [major|minor|patch]

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    print_error "GitHub CLI (gh) is not installed. Please install it first."
    exit 1
fi

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Get release type from argument
RELEASE_TYPE=${1:-patch}

if [[ "$RELEASE_TYPE" != "major" && "$RELEASE_TYPE" != "minor" && "$RELEASE_TYPE" != "patch" ]]; then
    print_error "Invalid release type. Use: major, minor, or patch"
    exit 1
fi

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
print_info "Latest tag: $LATEST_TAG"

# Parse version numbers
VERSION=${LATEST_TAG#v}
MAJOR=$(echo $VERSION | cut -d. -f1)
MINOR=$(echo $VERSION | cut -d. -f2)
PATCH=$(echo $VERSION | cut -d. -f3)

# Calculate new version
case $RELEASE_TYPE in
    major)
        NEW_MAJOR=$((MAJOR + 1))
        NEW_MINOR=0
        NEW_PATCH=0
        ;;
    minor)
        NEW_MAJOR=$MAJOR
        NEW_MINOR=$((MINOR + 1))
        NEW_PATCH=0
        ;;
    patch)
        NEW_MAJOR=$MAJOR
        NEW_MINOR=$MINOR
        NEW_PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="v${NEW_MAJOR}.${NEW_MINOR}.${NEW_PATCH}"
print_info "New version will be: $NEW_VERSION"

# Check for uncommitted changes
if [[ -n $(git status --porcelain) ]]; then
    print_warn "You have uncommitted changes. Commit them first? (y/n)"
    read -r response
    if [[ "$response" == "y" ]]; then
        git add .
        print_info "Enter commit message:"
        read -r commit_msg
        git commit -m "$commit_msg"
        git push origin main
    else
        print_error "Please commit or stash your changes before releasing"
        exit 1
    fi
fi

# Make sure we're on main branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$CURRENT_BRANCH" != "main" ]]; then
    print_warn "You're not on main branch. Switch to main? (y/n)"
    read -r response
    if [[ "$response" == "y" ]]; then
        git checkout main
        git pull origin main
    else
        print_error "Releases should be created from main branch"
        exit 1
    fi
fi

# Pull latest changes
print_info "Pulling latest changes..."
git pull origin main

# Run tests
print_info "Running tests..."
go test ./... || {
    print_error "Tests failed. Fix them before releasing."
    exit 1
}

# Update version in go.mod if needed
print_info "Creating release $NEW_VERSION..."

# Generate release notes
print_info "Generating release notes..."
RELEASE_NOTES=$(cat <<EOF
# Release $NEW_VERSION

## What's Changed

$(git log ${LATEST_TAG}..HEAD --pretty=format:"- %s" --reverse)

## Statistics

- Commits since last release: $(git rev-list ${LATEST_TAG}..HEAD --count)
- Contributors: $(git shortlog -sn ${LATEST_TAG}..HEAD | wc -l)
- Files changed: $(git diff --stat ${LATEST_TAG}..HEAD | tail -1)

## Quick Start

\`\`\`bash
# Clone and run
git clone https://github.com/c3stream/go-async-practice.git
cd go-async-practice
go run cmd/runner/main.go
\`\`\`

## Full Changelog

[${LATEST_TAG}...${NEW_VERSION}](https://github.com/c3stream/go-async-practice/compare/${LATEST_TAG}...${NEW_VERSION})
EOF
)

# Create git tag
print_info "Creating git tag..."
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

# Push tag
print_info "Pushing tag to GitHub..."
git push origin "$NEW_VERSION"

# Create GitHub release
print_info "Creating GitHub release..."
gh release create "$NEW_VERSION" \
    --title "Release $NEW_VERSION" \
    --notes "$RELEASE_NOTES" \
    --draft=false

print_info "✅ Release $NEW_VERSION created successfully!"
print_info "View at: https://github.com/c3stream/go-async-practice/releases/tag/$NEW_VERSION"

# Update CHANGELOG.md if it exists
if [[ -f "CHANGELOG.md" ]]; then
    print_info "Update CHANGELOG.md manually with the new release information"
fi

print_info "🎉 Release complete!"