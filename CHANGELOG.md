# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.0.0] - 2024-12-19

### 🎉 Major Release: Interactive Battle System & Advanced Challenges

This is a major release that expands the learning environment with interactive features and advanced distributed systems challenges.

### ✨ Added
- **Interactive Battle Arena System** - Real-time competitive learning with concurrent programming skills
  - Player vs Player battles using goroutines and channels
  - Skill system with cooldowns and damage calculation
  - XP and rating system for progression tracking
  - Real-time battle event logging

- **4 New Advanced Challenges (29-32)**
  - Challenge 29: Actor Model - Message-driven concurrency with supervision trees
  - Challenge 30: Reactive Streams - Backpressure-aware stream processing
  - Challenge 31: Distributed Transaction Coordinator - 2PC protocol with ACID guarantees
  - Challenge 32: Raft Consensus Algorithm - Leader election and log replication

- **Comprehensive Learning System**
  - XP and leveling system with achievements
  - Daily challenges and leaderboards
  - Performance metrics tracking
  - Progress persistence

- **Tutorial System**
  - Step-by-step interactive tutorials
  - Guided learning paths
  - Hands-on exercises with validation

- **Real-time Visualizer**
  - Goroutine lifecycle visualization
  - Channel flow monitoring
  - Lock contention heatmaps
  - Memory usage graphs

### 🔄 Changed
- Updated challenge count from 28 to 32
- Enhanced main menu with new interactive options
- Improved documentation for all challenges
- Expanded practical examples

### 📚 Documentation
- Updated README.md with new features
- Updated README_ja.md (Japanese) with complete translations
- Added comprehensive feature descriptions
- Enhanced quick start guides

### 📊 Statistics
- Total Challenges: 32
- Total Solutions: 24 × 3 patterns = 72
- Total Practical Examples: 13
- New Code Lines: 6,300+
- Total Project Size: 30,000+ lines

## [2.0.0] - 2024-12-15

### Added
- Challenges 17-28: Enterprise patterns
- Docker integration with 15+ services
- Comprehensive test suites
- Solutions for challenges 1-16

### Changed
- Restructured project layout
- Enhanced evaluation system
- Improved benchmarking

## [1.0.0] - 2024-12-01

### Added
- Initial 16 challenges
- Basic examples (1-16)
- Evaluation system
- Benchmark suite

### 🚀 Quick Start

```bash
# Try new challenges
go run cmd/runner/main.go -mode=challenge -challenge=29  # Actor Model
go run cmd/runner/main.go -mode=challenge -challenge=30  # Reactive Streams

# Battle Arena
go run cmd/runner/main.go -mode=battle

# Interactive Learning
go run cmd/runner/main.go -mode=interactive
```

### 🔗 Links
- [GitHub Repository](https://github.com/c3stream/go-async-practice)
- [Issue Tracker](https://github.com/c3stream/go-async-practice/issues)
- [Documentation](https://github.com/c3stream/go-async-practice/blob/main/README.md)