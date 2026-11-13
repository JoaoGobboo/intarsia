# AGENTS.md — Intársia

Instruções permanentes para agentes de programação que trabalham neste repositório.
Leia este arquivo inteiro antes de qualquer tarefa. Em caso de conflito entre este
arquivo e um pedido no chat, o pedido no chat vence; se o pedido contrariar uma
regra marcada como **inviolável**, pare e pergunte.

## O que é o Intársia

Uma aplicação web aberta (Apache 2.0), instalada na infraestrutura da própria
empresa, que unifica bancos de dados, planilhas e APIs e devolve o resultado como
API, planilha ou banco de dados. Opcionalmente, constrói uma ontologia por cima.

Recorte do produto: leve, uma máquina só, um binário. Não usamos Kubernetes,
microsserviços, filas externas nem servidores de banco obrigatórios. Qualquer
proposta que exija esse tipo de infraestrutura está fora do escopo.

## Idioma

- Converse com o usuário em português.
- Código, identificadores, comentários, mensagens de commit, mensagens de erro e
  documentação pública: em inglês.
- Chaves dos arquivos de projeto (YAML): em inglês.
- Textos da interface: nunca fixos no código. Sempre via arquivos de tradução,
  com português e inglês.

## Pilha

| Parte | Escolha |
| --- | --- |
| Linguagem do núcleo | Go |
| Motor de consulta | DuckDB embutido, pelo cliente oficial `github.com/duckdb/duckdb-go` (usa cgo) |
| Dados | Arquivos Parquet e um arquivo de catálogo DuckDB, em `data/` |
| Estado operacional | SQLite (usuários, chaves, execuções, registro de alterações) |
| Linha de comando | cobra |
| API | Servidor HTTP da biblioteca padrão do Go |
| Arquivos de projeto | YAML, com preservação de comentários ao regravar |
| Interface | React + TypeScript, página única, embutida no binário com `go:embed` |

Antes de adicionar qualquer dependência nova, confirme na documentação oficial o
caminho do módulo e a versão estável atual. Não fixe versões de memória. Não
adicione dependência para algo que a biblioteca padrão resolve.

## Estrutura do repositório

```
intarsia/
  cmd/intarsia/      # o binário: aplicação web, API de dados e comandos
  internal/
    project/         # lê, valida e grava os arquivos de projeto
    engine/          # DuckDB, catálogo e execução
    model/           # visual para SQL, dependências, linhagem
    ontology/        # compilador de objetos
    output/          # saídas, instantâneos e escrita
    server/          # aplicação web (API interna)
    dataapi/         # API de dados, processo separado
    connectors/      # conectores embutidos
  sdk/               # pacote público para escrever conectores
  web/               # interface em React
  demo/              # empresa fictícia e ambiente Docker
  docs/
```

Crie um pacote só quando a tarefa em andamento precisar dele. Não crie pastas
vazias nem código "para o futuro".

## Regras de arquitetura (invioláveis)

1. **Portas e adaptadores.** Os pacotes de núcleo (`project`, `model`,
   `ontology`, `output`) definem interfaces e não importam DuckDB, HTTP nem
   conector algum. A infraestrutura (`engine`, `server`, `dataapi`,
   `connectors`) implementa essas interfaces.
2. **`sdk/` é a única API pública de Go.** Tudo em `internal/` pode mudar.
3. **Conectores embutidos só importam `sdk/`.** Nunca outro pacote de
   `internal/`. Um conector embutido não tem privilégio sobre um plugin externo.
4. **A fonte da verdade são os arquivos de projeto.** A interface e a linha de
   comando leem e gravam os mesmos arquivos. Nada do que o usuário define pode
   existir só no banco de estado ou só na tela.
5. **Fontes são somente leitura por padrão.** Nenhuma conexão escreve numa fonte
   sem `writable: true` no arquivo dela.
6. **Credenciais nunca em arquivo nem em log.** O YAML referencia variáveis
   (`${NOME}`). Não imprima valores resolvidos, nem em mensagens de erro.
7. **Um arquivo por item.** Cada fonte, modelo, objeto e saída tem o seu
   arquivo, e o nome do arquivo é o nome do item.

## Formato dos arquivos de projeto

```
meu-projeto/
  project.yaml      # nome e versão do formato
  sources/          # um arquivo por fonte
  models/           # nome.visual.yaml ou nome.sql
  ontology/         # um arquivo por objeto
  outputs/          # um arquivo por saída
  data/             # cache e instantâneos; nunca vai para o Git
```

Exemplo de fonte:

```yaml
# sources/crm.yaml
type: postgres
mode: attach            # attach ou sync
connection:
  host: crm.interno
  database: crm
  user: ${CRM_USER}
  password: ${CRM_PASSWORD}
tables:                 # omitido = todas
  - clientes
```

Regras do formato:

- Os campos dentro de `connection` variam por tipo de conector; os demais são
  iguais para todas as fontes.
- `description` é opcional em todos os arquivos.
- O formato tem versão (`format:` em `project.yaml`). Mudança incompatível exige
  nova versão e migração.
- Ao regravar um arquivo, preserve comentários, ordem das chaves e formatação.

## Padrões do backend

- **Estado desejado e reconciliação:** os arquivos descrevem o que deve existir;
  a ferramenta compara, monta um plano e executa.
- **Compilador em etapas:** ler, validar, resolver referências, gerar SQL.
- **Grafo de tarefas** executado dentro do processo, sem fila externa.
- **Leitura separada da escrita:** a aplicação web produz instantâneos; a API de
  dados só os consome.
- **Registro só com acréscimos** para alterações feitas pela API.
- Erros de validação sempre informam arquivo, linha e campo.
- Sem estado global. Dependências entram por construtor.

## Padrões do frontend

```
web/src/
  app/         # inicialização, rotas, provedores
  features/    # sources, models, ontology, outputs, explorer
  shared/      # api, schema, ui, i18n, lib
```

Cada área de `features/` tem `api/`, `domain/`, `components/`, `routes/` e
`index.ts`.

Regras de dependência (verificadas por lint):

- `app` usa `features`, que usa `shared`. Nunca o contrário.
- Uma área só acessa outra pelo `index.ts` dela.
- Nada em `domain/` importa React.

Demais padrões:

- Dados da API passam por cache de requisições. Sem armazenamento global.
- Tipos TypeScript e formulários são gerados a partir dos esquemas publicados
  pelo servidor. Não declare à mão um tipo que o esquema já define.
- Formulários de fontes são montados pelo esquema, com um registro que liga
  tipo de campo a componente.
- Editores usam redutor de ações com histórico (`past`, `present`, `future`),
  guardando o modelo inteiro a cada ação, para desfazer e refazer.
- Componentes de tela não buscam dados; recebem prontos de funções da área.
- Quadro e editor de código são carregados sob demanda.

## Como trabalhar

- Uma tarefa por vez, pequena, com critério de pronto. Não amplie o escopo. Se
  notar algo fora da tarefa que precisa de atenção, descreva no final em vez de
  corrigir.
- Antes de escrever código, liste em poucas linhas o que vai fazer e quais
  arquivos vai tocar.
- Toda lógica nova vem com teste. Para o compilador e os arquivos de projeto,
  use arquivos de referência: a entrada e a saída esperada guardadas lado a lado
  em `testdata/`.
- Testes que dependem de banco usam a empresa fictícia em `demo/`.
- Rode build, testes e lint antes de declarar a tarefa concluída, e mostre o
  resultado. Se algo falhar e você não conseguir resolver, diga isso com
  clareza; não declare pronto.
- Não invente decisões de produto. Se a tarefa depender de algo que este arquivo
  ou a tarefa não definem, pare e pergunte.

## Comandos

Definidos no `Makefile` (criado na tarefa 1 da etapa 1):

| Comando | O que faz |
| --- | --- |
| `make build` | Compila a interface e o binário |
| `make test` | Testes de Go e do frontend |
| `make lint` | Lint de Go e do frontend, incluindo as regras de fronteira |
| `make dev` | Sobe o servidor e a interface em modo de desenvolvimento |
| `make demo` | Sobe a empresa fictícia |

Se um comando desta tabela ainda não existir, a tarefa que precisa dele o cria.

## O que não fazer

- Não introduzir linguagem de template em SQL (nada no estilo do dbt).
- Não gravar posição de caixas do quadro nos arquivos de projeto.
- Não expor a aplicação fora de `127.0.0.1` enquanto não houver login.
- Não aceitar SQL que altere dados nas consultas do usuário.
- Não adicionar telemetria, chamadas a serviços externos nem download de código
  em tempo de execução.
- Não reescrever arquivos que a tarefa não pede.
