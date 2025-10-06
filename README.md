# 🌀 Concurrent HTTP Load Balancer in Go

![GitHub Repo Size](https://img.shields.io/github/repo-size/JoYBoy7214/Load-Balancer-with-Go)
![GitHub last commit](https://img.shields.io/github/last-commit/JoYBoy7214/Load-Balancer-with-Go)
![GitHub issues](https://img.shields.io/github/issues/JoYBoy7214/Load-Balancer-with-Go)
![GitHub license](https://img.shields.io/github/license/JoYBoy7214/Load-Balancer-with-Go)

🚀 **A high-performance, fault-tolerant HTTP load balancer written in Go**, supporting multiple backend servers and concurrent request handling.

---

## 📌 Features

- **Load Balancing Algorithms**  
  - ✅ Round Robin  
  - ✅ Least Connections  

- **Fault Tolerance**  
  - Real-time health checks to detect and bypass unhealthy backends  
  - Automatic retry logic for failed requests  

- **High Concurrency**  
  - Atomic operations and mutexes ensure concurrency safety  
  - Handles multiple simultaneous requests efficiently  

- **Configurable via JSON**  
  - Backend servers and algorithm preferences are loaded from a `config.json` file  

---

## 🛠️ Tech Stack

- **Language:** Go (Golang)  
- **Core Concepts:** Concurrency, HTTP Networking, Atomic Operations, Mutexes  
- **Configuration:** JSON  

---

## 📥 Installation

```bash
# Clone the repository
git clone https://github.com/JoYBoy7214/Load-Balancer-with-Go.git
cd Load-Balancer-with-Go

# Build the project
go build -o loadbalancer

# Run the load balancer
./loadbalancer
```

---

## ⚡ Usage

1. **Configure Backend Servers**  
   Edit the `config.json` file to specify backend servers and load balancing algorithm.  

   Example `config.json`:
   ```json
   {
    "port": 3030,
    "strategy": "least-connections",
    "backends": [
      {"url": "http://localhost:8081", "weight": 2},
      {"url": "http://localhost:8082", "weight": 1}
    ]
   }
   ```

2. **Start Backend Servers**  
   Make sure your backend servers (e.g., simple Go HTTP servers) are running on the specified ports.

3. **Run the Load Balancer**  
   ```bash
   ./loadbalancer
   ```

4. **Send Requests**  
   Send HTTP requests to the load balancer:
   ```bash
   curl http://localhost:8080
   ```

   Traffic will be distributed based on the configured algorithm.

---

## 🧠 Future Enhancements

- Implement sticky sessions  
- Add a web dashboard for live monitoring  

---

## 👨‍💻 Author

Gowtham Balaji  

---

## 📄 License

This project is licensed under the **MIT License**.
