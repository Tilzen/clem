# PRD — GitHub como Coordination Backend

**Status:** Draft
**Autor:** Tilzen
**Data:** 2026-06-05
**Componente:** `internal/coordination`, `internal/runner`, `internal/watchdog`, `internal/config`

---

## 1. Resumo

Adicionar `github` como uma terceira opção de `coordination.backend` no `clem.yaml`, ao
lado de `discord` (default) e `slack`. Com isso, agentes passam a receber tarefas, reivindicá-las
e reportar status usando primitivas nativas do GitHub (Issues, labels, assignees, comentários)
em vez de um servidor de chat.

O backend já é uma abstração plugável (`coordination.Known()`), então o custo não está em
"adicionar a opção" — está em **substituir o mecanismo de push de notificação** que hoje é
específico do Discord.

## 2. Motivação

- **Os agentes já possuem credencial GitHub.** Cada agente abre PRs próprios (identidade git
  isolada por usuário Linux). Auth de coordenação fica resolvida sem segredo novo.
- **Fecha o loop tarefa → PR.** Hoje a tarefa vive no Discord/Slack e o resultado no GitHub,
  desconectados. Com Issues como board, issue e PR ficam linkados nativamente (`Closes #123`),
  com rastreabilidade e auditoria muito superiores a um fórum de chat.
- **Claim mais confiável.** O `assignee` é um primitivo melhor para reivindicação do que parsear
  prefixo de mensagem (`[TODO]`) em chat. Reduz o risco — hoje não especificado — de dois
  agentes pegarem a mesma tarefa.
- **Menos infra de MCP.** O README já recomenda `gh` CLI direto (mais econômico em tokens) em
  vez do GitHub MCP. O backend GitHub pode dispensar servidor MCP de coordenação.

## 3. Não-objetivos

- Substituir Discord/Slack como default. GitHub é uma opção adicional.
- Interface conversacional humano↔agente em tempo real (o ponto forte do chat permanece no chat).
- Suporte a GitLab/Bitbucket/Gitea (fora de escopo; o design não deve impedir, mas não entrega).
- Webhooks de entrada / servidor HTTP exposto (ver Fase 2; o MVP usa polling de saída).

## 4. Estado atual (o que o código já faz)

A abstração `coordination.Backend` (`internal/coordination/coordination.go`) é um registry:
`Known(name)` → struct por plataforma. Mas apenas parte dos campos é consumida de fato.

| Campo do `Backend`       | Consumido? | Onde |
|--------------------------|------------|------|
| `AlertTemplate` + `TokenEnvVar` | ✅ Sim | watchdog (`watchdog.go:275`) e alerta de CLAUDE.local.md (`runner.go:416`) — um `curl` POST |
| `MCPName` / `MCPBinary`  | ⚠️ Mortos | `.mcp.json` é gerado com branches Python **hardcoded** (`runner.go:69-98`: `discord-bot`, `slack-mcp`), não lê o struct |
| `TaskBoardNotes`         | ⚠️ Morto  | Definido, nunca injetado. A instrução real do board está escrita à mão no `CLAUDE.shared.md` (template do `init.go`) |

Mecanismo de notificação (push): `discordWatchChannels()` (`runner.go:624`) alimenta
`DISCORD_WATCH_CHANNELS` + `CLEM_TMUX_TARGET` (`runner.go:71-74`). O gateway watcher embutido no
`mcp-discord` observa os canais e faz `tmux send-keys` para **acordar a sessão** quando chega
tarefa. **Não existe equivalente GitHub pronto.**

## 5. Pontos de integração afetados

1. `internal/coordination/coordination.go` — novo `var github = Backend{}` + `case "github"` em `Known()`.
2. `internal/config/config.go` — validação (já chama `Known()` em `:926`); aceitar campos do board GitHub.
3. `internal/watchdog/watchdog.go` — alerta via `AlertTemplate` GitHub (comentário em issue).
4. `internal/runner/runner.go` — geração do `.mcp.json` (no-op para GitHub: usa `gh` CLI) e
   substituição do ramo discord-específico de "watch channels".
5. `cmd/init.go` + `CLAUDE.shared.md` — reescrever a semântica do board para primitivas GitHub.
6. `samples/` — novo sample `github-*/clem.yaml`.

## 6. Requisitos funcionais

### 6.1 Modelo de board (mapeamento)

| Conceito clem        | GitHub |
|----------------------|--------|
| Thread de tarefa     | Issue |
| Status `[TODO]` → `[IN PROGRESS]` → `[DONE]`/`[BLOCKED]` | Labels (`clem:todo`, `clem:in-progress`, `clem:done`, `clem:blocked`) |
| Claim                | `assignee` (self-assign) |
| Report de status     | Comentário na issue |
| Output do trabalho   | PR linkado (`Closes #N`) |
| Canal `#alerts`      | Issue dedicada de alertas (ou label `clem:alert`) |
| Canal `#lessons`     | Issue/Discussion de post-mortems |

### 6.2 Config (`clem.yaml`)

```yaml
coordination:
  backend: github
  # repo onde vive o board (owner/name). Pode ser o próprio repo de trabalho.
  github_repo: "Tilzen/clem-tasks"
  channels:
    # para GitHub, "channel" = número de issue OU label de roteamento.
    alerts:  "12"            # issue de alertas onde o watchdog comenta
    tasks:   "clem:todo"     # label que marca tarefas reivindicáveis
    lessons: "34"            # issue de post-mortems
```

### 6.3 Protocolo de claim (mitigação de corrida)

GitHub não oferece compare-and-swap em label/assignee. O prompt do agente DEVE:

1. Listar issues abertas com label de tarefa e **sem assignee**.
2. Escolher uma, fazer self-assign (`gh issue edit N --add-assignee @me`).
3. **Reler** a issue. Se o assignee confirmado for o próprio agente, prosseguir;
   caso contrário, abandonar e escolher outra.
4. Trocar label `clem:todo` → `clem:in-progress` ao começar.

### 6.4 Alerta (watchdog + CLAUDE.local.md)

`AlertTemplate` para GitHub: `POST /repos/{repo}/issues/{n}/comments` via `curl` com
`Authorization: Bearer $GITHUB_TOKEN`, onde `{n}` vem de `channels.alerts`. `TokenEnvVar: GITHUB_TOKEN`.

### 6.5 Descoberta de tarefas (notificação)

- **MVP:** polling no loop do runner. O agente, a cada iteração (`SLEEP_ACTIVE`/`SLEEP_NIGHT`),
  roda `gh issue list --label clem:todo --assignee "" --state open`. Sem infra nova.
- **Fase 2 (paridade com Discord):** watcher sidecar que faz polling da API do GitHub
  (`/notifications` ou issues com `since`/ETag), debounce, e `tmux send-keys` para acordar a
  sessão antes do próximo ciclo. Requer unit systemd + liberação de egress.

## 7. Requisitos não-funcionais

- **Egress firewall:** o tráfego para `api.github.com` precisa passar pelo proxy loopback /
  ser permitido pelas regras nftables por-UID. Agentes já alcançam o GitHub para PRs; o watcher
  da Fase 2, rodando como processo separado, exige sua própria liberação de egress.
- **Rate limit:** 5000 req/h por token autenticado. Polling por agente com ETag/`since` fica
  folgado. Documentar o intervalo mínimo recomendado.
- **Sem segredo novo em plaintext:** reusar o `GITHUB_TOKEN`/credencial já provisionada por agente.

## 8. Plano de entrega

### Fase 1 — MVP (polling, ~1 dia)
- [ ] `var github = Backend{}` + `case "github"` em `Known()`.
- [ ] `AlertTemplate` GitHub + `TokenEnvVar: GITHUB_TOKEN`.
- [ ] Validação de config (`github_repo`, semântica de `channels`).
- [ ] `.mcp.json`: branch GitHub é no-op (usa `gh` CLI).
- [ ] Reescrever seção de task-board do `CLAUDE.shared.md` para primitivas GitHub + protocolo de claim.
- [ ] Sample `samples/github-*/clem.yaml`.
- [ ] Testes unitários (`coordination`, `config`, `runner`).

### Fase 2 — Paridade event-driven (~1–3 dias)
- [ ] Watcher sidecar (poll API + debounce + `tmux send-keys`).
- [ ] Unit systemd + liberação de egress para `api.github.com`.
- [ ] Substituir o ramo `discordWatchChannels()` por um seletor por backend.
- [ ] Teste e2e (`.github/workflows/e2e.yml`).

### Limpeza (oportunística)
- [ ] Decidir destino dos campos mortos `MCPName`/`MCPBinary`/`TaskBoardNotes`: tornar os
      branches do runner data-driven ou remover os campos não usados.

## 9. Riscos e mitigações

| Risco | Mitigação |
|-------|-----------|
| Corrida de claim (sem CAS) | Protocolo self-assign + releitura (6.3) |
| Egress bloqueado para api.github.com | Liberar no proxy/nftables; validar no provision |
| Rate limit em polling agressivo | ETag/`since`, intervalo mínimo documentado |
| Perda da interface conversacional | Não-objetivo; quem quer chat mantém Discord/Slack (ou usa modo híbrido) |
| Campos mortos do struct confundem manutenção | Limpeza na entrega |

## 10. Alternativa: modo híbrido (futuro)

GitHub Issues como fonte-de-verdade do board + Discord/Slack apenas para notificação/conversa.
Fora do escopo deste PRD, mas o design do backend não deve impedir.

## 11. Critérios de aceite

- `coordination.backend: github` passa na validação de config.
- Um agente provisionado descobre uma issue `clem:todo` não atribuída, reivindica via assignee,
  troca a label para `clem:in-progress`, executa, abre PR com `Closes #N` e comenta o resultado.
- Watchdog e alerta de CLAUDE.local.md postam comentário na issue de alertas configurada.
- Discord e Slack continuam funcionando sem regressão.
