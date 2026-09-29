# Especificación y Diseño de Arquitectura Distribuida

## Sistema de Casino Online

* Alex Saez
* Christian Gajardo
* Yoandri Villarroel

### 1. Información General

* **Nombre del Sistema:** Sistema de Casino Online
* **Dominio:** Juegos de azar en línea, gestión transaccional de créditos virtuales y auditoría en tiempo real.
* **Contexto Académico:** Sistemas Distribuidos y Escalables.
* **Propósito:** Sistema de casino online que permite a los usuarios participar en distintos juegos utilizando créditos virtuales, gestionar su saldo y consultar el historial de sus actividades. El sistema contempla juegos de casino generales, con especial consideración de juegos de cartas como Blackjack y Póker.

### 2. Diagrama de Arquitectura Distribuida (Mermaid)

```mermaid
flowchart TB
    %% ==========================================
    %% CAPA DE CLIENTES Y ACCESO
    %% ==========================================
    subgraph CapaClientes ["Capa de Clientes"]
        Player(["👤 Jugador (Player)"])
        Frontend["📱 Frontend Web / Mobile<br/><i>(TypeScript / React)</i>"]
        Player -->|HTTPS / WSS| Frontend
    end

    subgraph Perimetro ["Perímetro de Red"]
        Gateway["🛡️ API Gateway (Go / Envoy)<br/>• Reverse Proxy & Auth Validation<br/>• Terminación WebSockets & Rate Limiting"]
    end

    Frontend -->|HTTPS / WSS| Gateway

    %% ==========================================
    %% CAPA DE NEGOCIO Y PERSISTENCIA
    %% ==========================================
    subgraph CapaNegocio ["Capa de Servicios de Negocio y Persistencia (Database-per-Service)"]

        %% Dominio 1: Cuentas y Finanzas (Izquierda)
        subgraph DomFinanzas ["Dominio: Cuentas y Finanzas"]
            direction TB
            AuthSvc["Auth & Users Service (Go)<br/>• Autenticación y Perfiles"]
            DB_Users[("users_db<br/>(PostgreSQL)")]
            AuthSvc -.->|SQL/TCP| DB_Users

            WalletSvc["Wallet Service (Go)<br/>• Ledger ACID y Balances"]
            DB_Wallet[("wallet_db<br/>(PostgreSQL)")]
            WalletSvc -.->|SQL/TCP| DB_Wallet

            AuthSvc -->|gRPC: InitWallet| WalletSvc
        end

        %% Dominio 2: Motores de Juego (Centro)
        subgraph DomJuegos ["Dominio: Motores de Juego"]
            direction TB
            BJGameSvc["Blackjack Game Service (Go)<br/>• Estado de Manos y Reglas BJ"]
            DB_BJ[("bj_state_db<br/>(MongoDB)")]
            BJGameSvc -.->|Mongo Protocol| DB_BJ

            PokerGameSvc["Poker Game Service (Go)<br/>• Mesas, Turnos y Botes"]
            DB_Poker[("poker_state_db<br/>(MongoDB)")]
            PokerGameSvc -.->|Mongo Protocol| DB_Poker
        end

        %% Dominio 3: Auditoría y Estadísticas (Derecha)
        subgraph DomAuditoria ["Dominio: Auditoría y Estadísticas"]
            direction TB
            HistorySvc["History & Audit Service (Go)<br/>• Log Inmutable de Jugadas"]
            DB_History[("audit_history_db<br/>(MongoDB)")]
            HistorySvc -.->|Mongo Protocol| DB_History

            StatsSvc["Leaderboard & Stats Service (Go)<br/>• Rankings y Clasificaciones"]
            DB_Stats[("stats_db<br/>(MongoDB)")]
            StatsSvc -.->|Mongo Protocol| DB_Stats

            HistorySvc -->|gRPC: Stream de Eventos| StatsSvc
        end

    end

    %% ==========================================
    %% CONEXIONES GATEWAY -> SERVICIOS
    %% ==========================================
    Gateway -->|gRPC: Auth / Sesión| AuthSvc
    Gateway -->|gRPC: Operaciones Saldo| WalletSvc
    Gateway -->|gRPC / WS: Blackjack| BJGameSvc
    Gateway -->|gRPC / WS: Poker| PokerGameSvc
    Gateway -->|gRPC: Consultar Historial| HistorySvc
    Gateway -->|gRPC: Consultar Rankings| StatsSvc

    %% ==========================================
    %% COMUNICACIÓN INTER-SERVICIOS
    %% ==========================================
    BJGameSvc -->|gRPC: Hold / Settle Bets| WalletSvc
    PokerGameSvc -->|gRPC: BuyIn / Payout| WalletSvc

    BJGameSvc -->|gRPC: RecordHandAudit| HistorySvc
    PokerGameSvc -->|gRPC: RecordTableAudit| HistorySvc

    %% ==========================================
    %% ESTILOS VISUALES
    %% ==========================================
    classDef client fill:#08427b,stroke:#052e56,stroke-width:2px,color:#fff;
    classDef gateway fill:#0f52ba,stroke:#083170,stroke-width:2px,color:#fff;
    classDef service fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#fff;
    classDef database fill:#0284c7,stroke:#0369a1,stroke-width:2px,color:#fff;

    class Player,Frontend client;
    class Gateway gateway;
    class AuthSvc,WalletSvc,BJGameSvc,PokerGameSvc,HistorySvc,StatsSvc service;
    class DB_Users,DB_Wallet,DB_BJ,DB_Poker,DB_History,DB_Stats database;
```
