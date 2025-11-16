# Project-Wide Instructions for AI Agents

## 1. Project Overview

This document provides general instructions and rules for AI agents working on the **Music Room** project.

**Music Room** is an educational project focused on creating a complete mobile, connected, and collaborative music application. The project is divided into three main components, located in their respective directories:

-   `/backend`: Contains all the server-side services and logic.
-   `/frontend`: Contains the web client application.
-   `/mobile`: Contains the mobile application.

This file (`/Agents/AGENTS.md`) contains the **global rules** that apply across the entire project. More specific instructions, context, or rules for each component can be found within the `Agents` folder inside that component's directory (e.g., `/frontend/Agents/`, `/backend/Agents/`).

The canonical description of the project goals and scope lives in `Agents/subject.txt`; treat that document as the source of truth whenever requirements conflict.

## 2. General Rules for Agents

All agents must adhere to the following rules throughout the project:

1.  **Language:** All documentation, code comments, commit messages, and any other written text must be in **English**.

2.  **Follow Conventions:** Strictly adhere to the existing coding style, formatting, naming conventions, and architectural patterns found in the specific part of the project you are working on.

3.  **Analyze Before Coding:** Before writing or modifying code, analyze the surrounding files, existing tests, and documentation to understand the established practices.

4.  **Dependencies:** Do not introduce new third-party libraries or dependencies without first verifying if they are appropriate for the project. Dependencies must be managed using the existing mechanisms (e.g., `go.mod`, `Makefile`).

5.  **Security:** Never commit API keys, credentials, or any other sensitive information to the repository. As per the project requirements, these must be stored locally in `.env` files and listed in `.gitignore`.

6.  **Testing:** When adding new features or fixing bugs, you must also add or update corresponding tests to ensure code quality and correctness.

7.  **Documentation:** Ensure that any new code is clearly documented where necessary, following the project's existing documentation style. All documentation must be in English.

### 2.1. Documentation Creation

-   **Format:** All documentation must be created as Markdown files (`.md`).
-   **Filename Convention:** Documentation filenames must be in all-caps with a lowercase extension (e.g., `ARCHITECTURE.md`, `DATABASE_SCHEMA.md`).
-   **Diagrams and Schemas:** For any diagrams, file relationships, or similar visual representations, you must use Mermaid syntax.
