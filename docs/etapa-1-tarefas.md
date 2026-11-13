# Intársia — Etapa 1: Unificar

Sete tarefas, em ordem. Cada seção é um pedido completo: cole uma por vez no
agente, revise o resultado e só então passe à seguinte. O `AGENTS.md` precisa
estar na raiz do repositório antes da primeira.

## Objetivo da etapa

Uma pessoa instala o Intársia, conecta um Postgres e uma planilha Excel e cruza
os dois pela tela, em cinco minutos.

## Fora do escopo da etapa 1

Modelos, saídas, API de dados, ontologia, login, escrita, conector REST,
SQL Server, Oracle, plugins externos e sincronização (`mode: sync`). Se uma
tarefa parecer exigir algum desses itens, o agente deve parar e avisar.

## Decisões a confirmar antes de começar

Estas quatro não foram fechadas no plano. As tarefas abaixo usam a proposta de
cada linha; mude aqui antes de começar se preferir outra.

| Decisão | Proposta usada nas tarefas |
| --- | --- |
| Formato de uma fonte de arquivo | `type: excel`, `csv`, `parquet` ou `pdf`, com `connection.path` |
| O que uma fonte `pdf` vira | Cada tabela detectada no PDF vira uma tabela consultável. Só PDF com texto; escaneado (OCR) fica fora |
| Onde ficam planilhas enviadas pela tela | Pasta `files/` dentro do projeto, versionada junto |
| Porta padrão da aplicação | 8080, alterável por opção de linha de comando |
| Caminhos da API interna | Os listados na tarefa 5, sob `/api/v1` |

---

## Tarefa 1 — Esqueleto do repositório

**Objetivo.** Ter um binário que compila nas três plataformas e abre uma página
vazia no navegador.

**Entregas.**

- `go.mod`, `cmd/intarsia/` com cobra e os comandos `version` e `serve`.
- `web/` com projeto React + TypeScript mínimo, uma página que mostra o nome e a
  versão, e a estrutura `app/`, `features/`, `shared/`.
- Interface compilada e embutida no binário com `go:embed`; `serve` entrega a
  página em `127.0.0.1:8080`.
- Tradução preparada, com português e inglês, mesmo havendo um texto só.
- `Makefile` com `build`, `test`, `lint` e `dev`.
- Lint de Go e do frontend, incluindo a regra de fronteiras entre `app`,
  `features` e `shared`, e a regra de que `internal/connectors` só importa `sdk`.
- Integração contínua que compila e testa em Linux, macOS e Windows.
- `LICENSE` (Apache 2.0), `README.md` curto e `.gitignore` (inclui `data/`).

**Pronto quando.**

- `make build` gera um binário único.
- `./intarsia version` imprime a versão.
- `./intarsia serve` abre a página em `http://127.0.0.1:8080`.
- `make test` e `make lint` passam, localmente e na integração contínua.

**Atenção.** O DuckDB ainda não entra nesta tarefa. Mas confirme que a
integração contínua tem compilador C disponível nas três plataformas, porque a
tarefa 4 vai precisar de cgo.

---

## Tarefa 2 — Arquivos de projeto

**Objetivo.** Ler, validar e gravar `project.yaml` e `sources/*.yaml`.

**Entregas.**

- Pacote `internal/project`, sem importar DuckDB nem HTTP.
- Tipos para o projeto e para a fonte, conforme o `AGENTS.md`. Tipos de fonte
  aceitos nesta etapa: `postgres`, `excel`, `csv`, `parquet`, `pdf`. Só
  `mode: attach`.
- Resolução de `${VARIAVEL}` a partir do ambiente, sem nunca registrar o valor.
- Validação com erros que informam arquivo, linha e campo.
- Gravação que preserva comentários, ordem das chaves e formatação.
- Esquema JSON de cada tipo de arquivo, gerado a partir dos tipos Go.
- Comandos `intarsia init` (cria a pasta de projeto) e `intarsia validate`.

**Pronto quando.**

- Testes com arquivos de referência em `testdata/` cobrem: projeto válido,
  campo desconhecido, tipo inválido, variável ausente e YAML malformado.
- Ler e regravar um arquivo com comentários não altera nenhum byte.
- `intarsia validate` num projeto com erro sai com código diferente de zero e
  mostra arquivo, linha e campo.

**Fora do escopo.** Modelos, objetos e saídas: se a pasta tiver esses arquivos,
ignore-os com um aviso.

---

## Tarefa 3 — Empresa fictícia

**Objetivo.** Ter dados realistas e sempre iguais para testes e demonstração.

**Entregas.**

- `demo/` com Docker Compose que sobe um Postgres.
- Gerador de dados em Go, com semente fixa: clientes no Postgres (o "CRM") e
  metas de vendas numa planilha Excel.
- Defeitos propositais: o mesmo cliente com grafias diferentes, chaves que não
  batem entre as fontes, datas em formatos distintos.
- Um projeto Intársia pronto em `demo/project/`, com as duas fontes declaradas.
- Comandos `intarsia demo up` e `intarsia demo down`, e `make demo`.

**Pronto quando.**

- `intarsia demo up` deixa o Postgres no ar e populado, e a planilha gerada.
- Rodar duas vezes produz exatamente os mesmos dados.
- `intarsia validate` passa no projeto de demonstração.

**Fora do escopo.** SQL Server, MinIO e a API REST falsa, que entram em etapas
seguintes. Deixe o gerador organizado para receber essas fontes depois.

---

## Tarefa 4 — Motor de consulta

**Objetivo.** Consultar as fontes do projeto com SQL, inclusive cruzando
Postgres e Excel.

**Entregas.**

- Pacote `internal/engine` com DuckDB embutido, atrás de uma interface definida
  pelo núcleo.
- Conexão a cada fonte do projeto: Postgres anexado em modo somente leitura;
  Excel, CSV e Parquet lidos do caminho declarado. Confirme na documentação do
  DuckDB a forma atual de anexar Postgres e de ler Excel.
- PDF: o DuckDB não lê PDF. As tabelas são extraídas em Go e entregues ao
  motor; cada tabela detectada vira uma tabela da fonte. Só PDF com texto; um
  PDF escaneado gera erro claro. A biblioteca de extração e o nome dado a cada
  tabela são confirmados com o usuário antes de implementar.
- Cada fonte vira um espaço de nomes: `crm.clientes`, `planejamento.metas`.
- Catálogo: lista de fontes, tabelas, colunas e tipos.
- Execução de consulta com limite de linhas e tempo máximo.
- Recusa de qualquer instrução que não seja consulta de leitura.
- Comandos `intarsia catalog` e `intarsia query "<sql>"`.

**Pronto quando.**

- Com a demonstração no ar, `intarsia query` devolve o resultado de um join
  entre `crm.clientes` e a planilha de metas.
- Um `insert`, `update`, `delete` ou `drop` é recusado com mensagem clara.
- Uma fonte fora do ar gera erro que nomeia a fonte, sem derrubar as outras.
- Os testes rodam contra a empresa fictícia.
- A compilação continua passando nas três plataformas.

**Atenção.** É a tarefa com mais risco técnico, por causa do cgo. Se a
compilação em alguma plataforma falhar e não houver solução simples, pare e
relate em vez de contornar.

---

## Tarefa 5 — API interna

**Objetivo.** Expor para a interface o que os comandos já fazem.

**Entregas.** Em `internal/server`, sob `/api/v1`:

| Método e caminho | O que faz |
| --- | --- |
| `GET /project` | Nome, versão do formato e resumo |
| `GET /sources` | Lista as fontes |
| `POST /sources` | Cria uma fonte (grava o arquivo) |
| `PUT /sources/{name}` | Altera uma fonte |
| `DELETE /sources/{name}` | Remove uma fonte |
| `POST /sources/{name}/test` | Testa a conexão |
| `POST /files` | Recebe uma planilha ou um PDF e grava em `files/` |
| `GET /catalog` | Fontes, tabelas e colunas |
| `GET /catalog/{source}/{table}/preview` | Prévia paginada |
| `POST /query` | Executa uma consulta de leitura |
| `POST /query/export` | Devolve o resultado em CSV, Excel ou Parquet |
| `GET /schemas/{kind}` | Esquema JSON de um tipo de arquivo |

- Especificação OpenAPI gerada, e cliente TypeScript gerado a partir dela em
  `web/src/shared/api`.
- Erros em formato único, com código, mensagem e, quando houver, o campo.
- Detecção de alteração concorrente: gravar uma fonte exige a versão lida; se o
  arquivo mudou por fora, a API recusa e informa.
- O servidor escuta só em `127.0.0.1`.

**Pronto quando.**

- Testes de API cobrem cada caminho, incluindo os erros.
- Criar uma fonte pela API produz um arquivo que passa em `intarsia validate`.
- Nenhuma resposta nem log contém valor de credencial.

---

## Tarefa 6 — Telas

**Objetivo.** Fazer tudo da etapa pela interface.

**Entregas.** Em `web/src/features/`:

- **Fontes (`sources`).** Lista, criação, edição e remoção. O formulário é
  montado pelo esquema de `GET /schemas/source`, com registro de campos. Botão
  de testar conexão. Envio de planilha ou PDF por arrastar e soltar, que cria a
  fonte.
- **Explorador (`explorer`).** Catálogo em árvore; prévia de tabela com rolagem
  virtual e paginação no servidor; editor SQL com execução, resultado em tabela
  e download em CSV, Excel e Parquet.
- Componentes comuns em `shared/ui`, com cores e espaçamentos em variáveis e
  tema claro e escuro.
- O endereço da página reflete a fonte, a tabela ou a consulta aberta.
- Editor de fonte com redutor de ações e histórico, para desfazer e refazer.
- Erros da API mostrados no campo a que se referem.
- Todos os textos em português e inglês.

**Pronto quando.**

- Com a demonstração no ar, é possível, só pela tela: criar a fonte Postgres,
  enviar a planilha, ver as duas no catálogo, escrever o join e baixar o
  resultado em Excel.
- Desfazer e refazer funcionam no formulário de fonte.
- `make lint` passa, incluindo as regras de fronteira.
- Testes de componente cobrem o formulário gerado e a tabela de resultados.

**Atenção.** Esta tarefa precisa de revisão humana na tela. O agente deve
entregar capturas de tela ou instruções claras de como ver cada parte.

---

## Tarefa 7 — Fechamento

**Objetivo.** Garantir que alguém de fora consegue usar.

**Entregas.**

- Teste de ponta a ponta automatizado: sobe a demonstração, abre a aplicação,
  conecta as duas fontes, executa o join e confere o resultado.
- `docs/primeiros-passos.md`, em português e inglês, do download ao primeiro
  join.
- `README.md` completo: o que é, instalação, demonstração em três comandos.
- Binários publicados para Linux, macOS e Windows a partir de uma etiqueta de
  versão.

**Pronto quando.**

- O teste de ponta a ponta passa na integração contínua.
- Uma pessoa que não participou do desenvolvimento segue o guia e chega ao
  join em cinco minutos, sem ajuda. Este critério é verificado por você, não
  pelo agente.

---

## Como revisar cada entrega

1. Os comandos de "Pronto quando" funcionam na sua máquina?
2. `make test` e `make lint` passam?
3. O agente tocou em arquivos fora do que a tarefa pedia?
4. Há dependência nova? É mesmo necessária?
5. Alguma regra inviolável do `AGENTS.md` foi contornada?

Se uma tarefa sair grande demais para revisar de uma vez, peça ao agente que a
divida em duas antes de continuar.
