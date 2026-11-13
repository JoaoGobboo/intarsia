# Intársia: plano da plataforma aberta de dados com ontologia

Nov 12, 2026 · @João

## Visão e posicionamento

O projeto é uma aplicação web aberta, instalada na infraestrutura da própria empresa, que unifica bancos de dados, planilhas e APIs e devolve o resultado como API, planilha ou banco de dados. Opcionalmente, constrói uma ontologia por cima. A inspiração é o Palantir Foundry, mas o recorte é outro: leve, numa máquina só e instalável com um comando.

O produto tem dois níveis, e o primeiro já entrega valor sozinho:

- **Nível 1, unificar.** O usuário conecta fontes e tudo vira dataset consultável com SQL, inclusive com joins entre uma planilha e um banco.
- **Nível 2, ontologia.** Um mapeamento declarativo transforma datasets em objetos de negócio (Cliente, Pedido), com links entre eles, uma API gerada e, mais adiante, ações de escrita.

O caso de uso central: três sistemas, cada um com seu banco, unificados e publicados como uma API para um workflow novo ou como uma planilha sempre atualizada.

A frase de posicionamento é: leve e instalado na sua própria infraestrutura. Os projetos existentes exigem Kubernetes; este é um binário numa máquina.

O nome do projeto é Intársia, a técnica de encaixar pedaços de madeiras diferentes para formar um desenho: fontes distintas, que mantêm sua origem e se encaixam num todo. A palavra é quase a mesma em português, inglês, italiano e alemão. No comando, no repositório e nos endereços usa-se `intarsia`, sem acento.

A licença é Apache 2.0. É permissiva, inclui proteção de patentes e é aceita sem resistência por departamentos jurídicos, o que importa para um público corporativo e para atrair quem escreve conectores. A AGPL foi descartada porque muitas empresas a proíbem internamente.

## Panorama: o que já existe

Já há projetos abertos nesse espaço, mas todos miram o ambiente enterprise e começam pela ontologia. A lacuna é o caso pequeno: um Postgres, algumas planilhas e duas APIs unificados sem montar um cluster.

| Projeto | Abordagem | Diferença para o nosso |
| --- | --- | --- |
| [OpenFoundry (DioCrafts)](https://github.com/DioCrafts/OpenFoundry) | Monorepo Go com 41 microsserviços, Kubernetes, licença AGPL-3.0 | Pesado; exige infraestrutura de cluster |
| [OpenFoundry (smilebank7)](https://github.com/smilebank7/OpenFoundry) | Compõe Airbyte, Iceberg, MinIO e Trino com uma camada de ontologia própria | Mesma ideia de composição, mas com pilha de data lake completa |
| [openfoundry (Przyval)](https://github.com/Przyval/openfoundry) | Emulador local das APIs do Foundry, dados em memória | Serve para testar apps Foundry, não para unificar dados reais |
| [DataSteward](https://github.com/Leonxu6/datasteward) | Implementação de referência com governança e agente de IA | Demonstração de conceito sobre Postgres, dbt e Dagster |
| [Semantica](https://themenonlab.blog/blog/semantica-open-source-palantir-alternative-ai-agents) | Camada de grafo de conhecimento para agentes de IA, licença MIT | Foco em IA, não em integração de dados |

Três conclusões:

- O nome "OpenFoundry" está saturado: há pelo menos três projetos distintos com ele.
- Ninguém atende bem o uso local e leve.
- O modelo de dois níveis, em que unificar já basta, é um diferencial.

Ressalva: a avaliação vem das descrições dos repositórios, lidas em novembro de 2026. Nenhum foi instalado ou testado, e a maturidade real é incerta.

## Usuários e fonte única da verdade

O projeto atende dois públicos, técnicos e analistas de negócio, e os dois editam os mesmos arquivos. Tudo que o usuário define (fontes, modelos, objetos, links) vive em arquivos declarativos numa pasta de projeto.

- **Técnico:** edita os arquivos YAML e SQL direto e versiona no Git.
- **Analista:** usa a interface web, que lê e grava esses mesmos arquivos.

Três regras sustentam isso:

1. O formato dos arquivos é a API mais importante do projeto. Precisa ser simples de escrever à mão e regular o bastante para a interface gerar sem bagunçar.
2. A interface não tem poderes exclusivos. Tudo que se faz na tela é representável em arquivo.
3. SQL é a ponte. O técnico escreve a transformação; o analista monta a mesma coisa num construtor visual que gera SQL visível.

A interface web é o produto principal e entra desde a primeira etapa, para conectar fontes e explorar dados. A linha de comando continua existindo para usuários técnicos e automação, sobre os mesmos arquivos.

## Arquitetura

A plataforma tem seis peças: projeto em arquivos, conectores, motor DuckDB, modelos, compilador de ontologia e saídas publicadas.

&#91;embedded content: arquitetura · seis peças em camadas\]

Os dados descem das fontes até as saídas, e os arquivos de projeto configuram conectores, modelos, ontologia e saídas. Uma saída pode partir direto de um modelo, sem ontologia: é o nível 1. A API de dados lê apenas os instantâneos publicados.

Um objeto da ontologia, na forma mais curta. Os campos completos estão na seção seguinte.

```yaml
# ontology/cliente.yaml
label: Cliente
source: models.clientes_unificados
key: cliente_id
properties:
  nome: {type: string}
links:
  pedidos: {to: pedido, via: cliente_id, many: true}
```

## Formato dos arquivos de projeto

O projeto é uma pasta com um arquivo por item: cada fonte, modelo, objeto e saída tem o seu. A interface regrava só o arquivo que mudou, o histórico no Git fica legível e edições paralelas não entram em conflito.

```
meu-projeto/
  project.yaml      # nome e versão do formato
  sources/          # um arquivo por fonte
    crm.yaml
    erp.yaml
  models/           # um arquivo por modelo
    clientes_unificados.visual.yaml
    margem_por_produto.sql
  ontology/         # um arquivo por objeto
    cliente.yaml
  outputs/          # um arquivo por saída
    pedidos_completos.yaml
  data/             # cache e instantâneos, fora do Git
```

O nome do arquivo vira o nome do item, e `description` é opcional em todos. Os nomes dos campos ficam em inglês; nomes de tabelas, objetos e descrições ficam no idioma do usuário. Agendas aceitam intervalo simples (`every 15m`) ou expressão cron.

### Fonte

Uma fonte declara tipo, modo, conexão e tabelas. As credenciais são sempre referências a variáveis.

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
  - name: contatos
    incremental: atualizado_em
sync:
  schedule: every 1h    # só no modo sync
```

- `connection`: os campos mudam conforme o tipo do conector; o resto é igual para todas as fontes.
- `incremental`: a coluna que indica o que mudou desde a última cópia.
- `writable: true`: marca a fonte como gravável, para servir de banco de destino ou receber escrita. Sem isso, a conexão é somente leitura.

### Objeto

Um objeto mapeia um modelo para um conceito de negócio, com propriedades e links.

```yaml
# ontology/cliente.yaml
label: Cliente
plural: Clientes
source: models.clientes_unificados
key: cliente_id
title: nome
properties:
  nome: {type: string}
  cnpj: {type: string, sensitive: true}
  criado_em: {type: date, column: dt_cadastro}
links:
  pedidos: {to: pedido, via: cliente_id, many: true}
```

- `title`: a propriedade que aparece como nome do objeto na tela.
- `column`: usado quando a propriedade tem nome diferente da coluna.
- `sensitive`: marca dados que poderão ser mascarados quando houver permissões.
- **Propriedades explícitas:** só o que for declarado aparece na API e na tela, para uma coluna nova do banco não ser exposta por acidente. A interface oferece um botão para adicionar todas.
- `actions`: chave reservada para as ações, na última etapa.

Os campos da saída estão em Saídas e uso dos dados.

### Modelos: visual primeiro, código quando preciso

Há dois jeitos de criar um modelo, seguindo a divisão que o Foundry faz entre [Pipeline Builder e Code Repositories](https://palantir.com/docs/foundry/learning-data-dataeng-01/10).

| Tipo | Arquivo | Para quem e quando | Reutilização |
| --- | --- | --- | --- |
| Visual (principal) | `nome.visual.yaml` | Todo modelo novo começa aqui, na interface; serve aos dois públicos | Copiar e colar passos ou modelos |
| SQL puro | `nome.sql` | Lógica que o visual não alcança | Modelo intermediário que os outros consultam |
| Python (futuro) | A definir | Lógica especializada: chamar API, usar biblioteca | Funções e bibliotecas |

O modelo visual é um grafo de passos gravado em YAML, que a ferramenta compila para SQL. Cada passo tem um identificador e recebe os dados do passo anterior, a menos que indique outra entrada. Um modelo linear é o caso simples, e continua curto de escrever.

```yaml
# models/pedidos.visual.yaml
steps:
  - id: clientes_ativos
    from: crm.clientes
    filter: status = 'ativo'

  - id: limpos
    from: erp.pedidos
  - id: com_cliente
    join:
      with: clientes_ativos
      type: left
      on: {cliente: id}

  - id: validos
    input: com_cliente
    filter: cliente is not null and valor > 0
    output: true
  - id: com_problema
    input: com_cliente
    filter: cliente is null or valor <= 0
    output: true
```

O exemplo tem dois ramos que se encontram na junção e depois se dividem em dois resultados. O `with` pode apontar para um passo do próprio modelo, uma tabela de fonte ou outro modelo.

| Passo | Chave | O que faz |
| --- | --- | --- |
| Origem | `from` | Define de onde vêm os dados |
| Colunas | `select` | Escolhe, renomeia e reordena colunas |
| Filtro | `filter` | Mantém as linhas que atendem a uma condição |
| Junção | `join` | Cruza com outra tabela por uma chave |
| Empilhar | `union` | Junta linhas de tabelas com o mesmo formato |
| Calcular | `derive` | Cria uma coluna a partir de outras |
| Limpar | `clean` | Corrige tipos, datas, espaços e maiúsculas |
| Deduplicar | `dedupe` | Mantém uma linha por chave, segundo uma regra |
| Agrupar | `group` | Soma, conta, tira média |
| Ordenar e limitar | `sort`, `limit` | Ordena e corta o resultado |
| SQL livre | `sql` | Trecho de SQL no meio do grafo, para o que faltar |

Pivotar e funções de janela ficam para uma versão posterior. As regras do modelo visual:

- **Duas visões, um arquivo.** A interface mostra o modelo como quadro (caixas e setas) ou como lista de passos, e as duas gravam o mesmo YAML.
- **Um bloco de SQL por passo.** Cada passo compila para um bloco nomeado da consulta. Isso dá SQL legível, prévia dos dados em cada passo e erro apontando o passo exato.
- **Expressões em SQL.** Condições e cálculos são expressões SQL. A interface oferece formulários para os casos comuns e gera a expressão.
- **Um ou vários resultados, à escolha do usuário.** Sem marcação, o resultado é o último passo, referenciado como `models.pedidos`. Com `output: true` em mais de um passo, cada um vira uma tabela: `models.pedidos.validos`.
- **Resultados trocados juntos.** O modelo roda como uma unidade. Se um passo falhar, nenhum resultado é atualizado.
- **Quadro organizado automaticamente.** As posições das caixas não são gravadas no arquivo, para não sujar o histórico do Git.

O mesmo quadro serve ao grafo do projeto (fontes, modelos, objetos e saídas), de onde se entra em cada modelo.

O modelo em SQL é uma consulta comum, que roda em qualquer editor. A ferramenta lê a consulta para descobrir as dependências, sem funções especiais:

```sql
-- description: Clientes do CRM com total de pedidos do ERP
-- materialize: table
select c.id as cliente_id, c.nome, count(p.id) as pedidos
from crm.clientes c
left join erp.pedidos p on p.cliente = c.id
group by all
```

Quatro decisões fecham o desenho:

- **Os tipos se juntam pelos datasets.** Qualquer modelo lê o resultado de qualquer outro como `models.nome`, e a linhagem mostra o pipeline inteiro.
- **Sem linguagem de template.** Não há macros no estilo do dbt. Lógica dinâmica vai para Python, quando existir.
- **Visual para SQL é caminho só de ida.** Um modelo visual pode ser convertido em SQL, mas SQL escrito à mão não volta a ser visual.
- **Python roda como processo separado** que grava Parquet, o mesmo contrato dos plugins de conectores. O binário em Go não embute Python.

Diferente do Foundry, onde cada ferramenta é uma aplicação à parte, aqui tudo fica na mesma pasta de projeto e na mesma tela.

## Pilha técnica

O núcleo é escrito em Go com DuckDB embutido, distribuído como um binário único que já contém a interface web.

| Parte | Escolha | Observação |
| --- | --- | --- |
| Linguagem | Go | Binário único; interface embutida com `go:embed` |
| Motor de consulta | DuckDB, via [duckdb-go](https://duckdb.org/docs/current/clients/go/overview) | Cliente oficial; usa cgo e liga a biblioteca estaticamente |
| Armazenamento dos dados | Arquivos Parquet e um arquivo de catálogo DuckDB | Sem servidor de banco para instalar |
| Estado operacional | SQLite | Usuários, chaves, execuções e registro de alterações |
| Dependências e linhagem | Árvore sintática em JSON gerada pelo próprio DuckDB | Substitui o sqlglot, que só existe em Python |
| Linha de comando | cobra |  |
| API | Servidor HTTP da biblioteca padrão |  |
| Arquivos de projeto | YAML com preservação de comentários | A interface regrava arquivos editados à mão |
| Interface | React | Compilada e embutida no binário |

O custo do cgo é a compilação cruzada mais trabalhosa. O cliente traz bibliotecas prontas para macOS e Linux (amd64 e arm64) e Windows (amd64), o que cobre o público-alvo.

## Arquitetura do código

O código é um monólito modular com portas e adaptadores, num repositório único. O núcleo (projeto, modelos, ontologia, saídas) define interfaces e não conhece DuckDB, HTTP nem conector algum; a infraestrutura implementa essas interfaces. Microsserviços estão descartados pelo posicionamento.

```
intarsia/
  cmd/intarsia/      # o binário: aplicação web, API de dados e comandos
  internal/
    project/         # lê e grava os arquivos de projeto
    engine/          # DuckDB, catálogo e execução
    model/           # visual para SQL, dependências, linhagem
    ontology/        # compilador de objetos
    output/          # saídas, instantâneos e escrita
    server/          # aplicação web
    dataapi/         # API de dados, o processo separado
    connectors/      # conectores embutidos
  sdk/               # pacote público para escrever conectores
  web/               # interface em React
  demo/              # empresa fictícia e ambiente Docker
  docs/
```

Quatro regras organizam o repositório:

- **Um repositório só.** Núcleo, interface, conectores e demonstração evoluem juntos; uma mudança no formato dos arquivos atualiza tudo num commit.
- `sdk/` **é a única parte pública.** É o contrato dos conectores. Tudo em `internal/` pode mudar sem quebrar ninguém.
- **Conectores embutidos só importam o** `sdk/`**.** Se um conector usar algo do núcleo, a compilação falha. Mover um conector para fora ou para dentro vira mudança de pasta.
- `demo/` **fica no repositório**, porque a empresa fictícia serve aos testes automáticos.

Plugins de terceiros ficam nos repositórios de quem os escreve, dependendo só do `sdk/`.

### Padrões

| Padrão | O que significa aqui | O que resolve |
| --- | --- | --- |
| Estado desejado e reconciliação | Os arquivos descrevem o que deve existir; a ferramenta compara com o que existe, monta um plano e executa | Prévia do que vai mudar antes de aplicar |
| Compilador em etapas | Modelos, objetos e saídas passam por ler, validar, resolver referências e gerar SQL, com uma representação interna comum | Visual, SQL e Python alimentam a mesma linhagem |
| Grafo de tarefas com executor | Sincronizar, materializar e publicar são tarefas num grafo, executadas em ordem e em paralelo, dentro do processo | Dispensa fila externa |
| Leitura separada da escrita (CQRS) | A aplicação web produz instantâneos; a API de dados só os consome | Isolamento entre modelagem e consumo |
| Registro só com acréscimos | Cada alteração feita pela API é uma linha nova, com autor e data; nada é sobrescrito | Histórico e regra de conflito |

Os testes fazem parte do desenho:

- **Conectores:** bateria de conformidade, igual para embutidos e externos.
- **Compilador:** arquivos de referência, em que cada YAML de entrada tem o SQL esperado guardado ao lado. Toda mudança no SQL gerado aparece na revisão.

O estado operacional fica em SQLite, e o DuckDB guarda só os dados. Usuários, chaves de acesso, histórico de execuções e registro de alterações são dados pequenos e gravados o tempo todo, e o DuckDB aceita um único processo gravando. Os dois são arquivos locais, sem servidor.

A linha de comando trabalha de dois jeitos: usa o núcleo diretamente para validar e compilar projetos, sem servidor no ar, e atua como cliente da aplicação web para o que depende de estado, como disparar uma atualização.

## Arquitetura do frontend

A interface é um editor dos arquivos de projeto e um visualizador dos dados: a verdade fica no servidor, e a tela guarda o mínimo de estado próprio. É uma aplicação de página única em React com TypeScript, sem renderização no servidor, compilada e embutida no binário.

```
web/src/
  app/         # inicialização, rotas, provedores
  features/
    sources/
    models/
    ontology/
    outputs/
    explorer/
  shared/
    api/       # cliente gerado a partir da API
    schema/    # esquemas e tipos gerados
    ui/        # componentes visuais comuns
    i18n/
    lib/
```

Cada área de `features/` tem a mesma forma:

```
features/models/
  api/         # consultas e gravações
  domain/      # lógica pura: grafo, validação
  components/  # componentes de tela
  routes/      # páginas
  index.ts     # o que a área expõe
```

Três regras de dependência, verificadas por lint desde o primeiro commit:

- `app` usa `features`, que usa `shared`. Nunca o contrário.
- Uma área só acessa outra pelo `index.ts` dela.
- A lógica em `domain/` não importa React.

### Padrões

| Padrão | Como funciona |
| --- | --- |
| Estado do servidor separado do estado da tela | O que vem da API passa por um cache de requisições; só o efêmero (zoom, painel aberto) é estado local. Sem armazenamento global |
| Um esquema, três usos | O servidor publica o esquema de cada tipo de arquivo. Dele saem a validação em Go, os tipos TypeScript e o autocompletar do editor de YAML |
| Formulários gerados pelo esquema | O formulário de uma fonte ou de um passo é montado a partir do esquema. Um conector externo ganha a tela sem trazer código React |
| Registro de campos | Uma tabela liga cada tipo de campo do esquema a um componente: texto, senha, lista, expressão SQL |
| Rascunho, validação, gravação | Editar cria um rascunho em memória; o servidor valida e devolve SQL, erros e prévia; só então grava. Se o arquivo mudou por fora, a tela avisa |
| Duas visões, um modelo | Quadro e lista desenham o mesmo grafo em memória e despacham as mesmas ações |
| Lógica pura fora do React | Operações sobre o grafo são funções TypeScript comuns, testadas sem navegador |
| Componentes sem lógica de dados | Buscar e gravar fica em funções da área, como `useModel`; o componente recebe dados prontos |
| Erros com endereço | A API devolve erros com o campo ou o passo a que se referem; cada área tem sua barreira de erro |
| Carregamento sob demanda | O quadro e o editor de código só são baixados quando a tela os usa |

### Desfazer e refazer

Toda mudança num rascunho passa por um único ponto: o componente despacha uma ação, e um redutor devolve um modelo novo, sem alterar o anterior. O histórico guarda o modelo inteiro a cada ação.

```ts
type History = {
  past: Model[];     // estados anteriores
  present: Model;    // o que está na tela
  future: Model[];   // o que foi desfeito
};
```

- **Editar:** o `present` vai para o `past`, o resultado vira `present` e o `future` é esvaziado.
- **Desfazer e refazer:** movem um estado entre `past`, `present` e `future`.
- **Ações agrupadas:** edições seguidas no mesmo campo contam como uma; um gesto, como arrastar uma ligação, também.
- **Só o conteúdo entra no histórico.** Zoom, seleção e painéis ficam de fora.
- **Validação fora do histórico.** Erros e SQL gerado são recalculados a partir do `present`.
- **Gravar não apaga o histórico**, que guarda as últimas cem ações.

Guardar o modelo inteiro é barato, porque um modelo tem dezenas de passos e as partes inalteradas são compartilhadas entre versões. Vale para os editores de modelo visual, objeto, saída e fonte, e precisa existir desde o início: sem o ponto único de mudança, acrescentar depois exige reescrever o editor.

### Escolhas de base

- **Dois idiomas desde o início:** português e inglês, com a interface preparada para tradução.
- **Componentes próprios sobre primitivos acessíveis**, com cores, espaçamentos e tipografia em variáveis. Tema claro e escuro saem daí.
- **Estilo com classes utilitárias** sobre as variáveis de tema.
- **Telas específicas feitas à mão**, como o quadro; o resto usa os formulários gerados.
- **Apoio:** tabela com rolagem virtual e paginação no servidor, editor de código leve para SQL e YAML, endereço da página refletindo o item aberto, e testes de ponta a ponta contra a empresa fictícia.

Bibliotecas candidatas: TanStack Query, React Flow, CodeMirror, Radix e Tailwind. As versões são confirmadas na montagem do projeto.

## Conectores e plugins

Um conector pode rodar embutido no binário ou como plugin externo, e trocar de lado é mudar uma linha. O autor escreve contra uma interface e nunca sabe em qual modo está rodando.

```go
type Connector interface {
    ListTables() ([]Table, error)
    Read(table string) (Rows, error)
}

// Externo: um main.go
func main() { sdk.Serve(salesforce.New()) }

// Embutido: uma linha no núcleo
registry.Add("salesforce", salesforce.New)
```

Quatro regras garantem a portabilidade:

- **Embutido não tem privilégio.** Conector interno só acessa o que a interface oferece.
- **Dados trafegam por arquivo.** O conector grava Parquet num caminho indicado pelo núcleo; o protocolo só carrega comandos pequenos.
- **Testes de conformidade.** A mesma bateria roda contra o conector nos dois modos.
- **Resolução única.** O usuário escreve `type: salesforce`; o programa procura nos embutidos e depois nos plugins instalados.

O transporte dos plugins externos usa a biblioteca go-plugin da HashiCorp. O ciclo de vida esperado: um plugin externo ganha uso, passa na conformidade e é promovido a embutido.

Há dois modos de acesso aos dados:

- **Attach:** consulta na fonte, sem copiar. Vale para bancos, via extensões do DuckDB, e fica fora do sistema de plugins.
- **Sync:** copia para Parquet local. Vale para APIs, planilhas e fontes sem extensão; é o que a interface `Connector` cobre.

Para APIs REST comuns há um conector genérico declarativo: o usuário descreve URL, autenticação e paginação em YAML, sem escrever código.

Limitação: plugin escrito em outra linguagem funciona como externo, mas não pode ser embutido.

## Fontes de dados

As fontes entram em três ondas, começando pelas que o DuckDB já cobre com [extensões oficiais](https://duckdb.org/docs/current/core_extensions/overview).

| Onda | Fonte | Como entra |
| --- | --- | --- |
| 1 | Postgres, MySQL, SQLite | Extensão oficial do DuckDB (attach) |
| 1 | Excel, CSV, Parquet, JSON | Leitura nativa do DuckDB |
| 1 | PDF (tabelas, só PDF com texto) | Extração nossa em Go |
| 1 | S3 e compatíveis | Extensão httpfs |
| 1 | APIs REST | Conector declarativo nosso (sync) |
| 2 | SQL Server, Oracle | Conector nosso em Go (sync); ODBC como alternativa |
| 2 | SharePoint, OneDrive | Conector nosso (sync) |
| 2 | Azure Blob Storage | Extensão oficial do DuckDB |
| 3 | Snowflake, BigQuery | Conector nosso (sync) |
| 3 | Databricks, Delta Lake, Iceberg | Extensões oficiais do DuckDB |
| 3 | Google Sheets, Google Drive | Conector nosso (sync) |
| 3 | Salesforce, MongoDB | Conector nosso (sync) |
| Plugins | SAP, TOTVS e outros ERPs | Pelo banco por baixo, por OData ou plugin externo |

A onda 1 valida a arquitetura inteira. A onda 2 é o que torna o projeto corporativo, porque SQL Server, Oracle e SharePoint são onde a maior parte dos dados de empresa está.

A extensão ODBC do DuckDB abre quase qualquer banco corporativo, mas exige que o usuário instale o driver, tem suporte de melhor esforço e parece recente. Precisa ser testada antes de virar dependência.

## Saídas e uso dos dados

Uma saída é a declaração do que o usuário quer publicar a partir de um modelo ou de um objeto da ontologia. A mesma definição gera a API e a planilha, sem programação.

```yaml
# outputs/pedidos_completos.yaml
source: models.pedidos.validos   # ou object: pedido
formats: [json, csv, xlsx]
columns: [id, cliente_id, valor, data]
filters: [cliente_id, data]
mode: cache             # cache, live ou database
refresh: every 15m
destination:            # só no modo database
  source: dw
  table: pedidos_completos
access: [workflow_vendas, equipe_bi]
```

- `columns`: omitido, publica todas as colunas.
- `destination`: aponta para uma fonte marcada como gravável, em vez de repetir a conexão. As credenciais ficam num lugar só.
- `access`: nomes das chaves autorizadas. As chaves são criadas na aplicação e nunca ficam em arquivo.

```
GET /api/pedidos_completos?cliente_id=42
GET /api/pedidos_completos.xlsx
```

O primeiro endereço alimenta um workflow. O segundo é a planilha, e a mesma URL pode ser usada como fonte de dados no Excel ou no Google Sheets, que se atualizam sozinhos.

Cada saída escolhe um de três modos de entrega:

| Modo | Como funciona | Quando usar | Custo |
| --- | --- | --- | --- |
| Cache (padrão) | Cópia interna atualizada no intervalo definido | Relatórios, planilhas, a maioria dos workflows | Dado com atraso de até um intervalo |
| Ao vivo | Consulta as fontes a cada chamada | Dado que precisa ser o do momento, como saldo de estoque | Resposta lenta, carga nas fontes, falha se uma fonte cair |
| Banco | Grava o resultado como tabelas num banco de destino, ou num arquivo SQLite, DuckDB ou dump SQL | Ferramentas de BI e sistemas que só falam SQL | Exige um destino com permissão de escrita |

Toda resposta informa de quando é o dado. Junto com as saídas vêm:

- **Documentação automática:** especificação OpenAPI, para n8n, Power Automate e similares.
- **Filtros e paginação:** só nos campos que a saída declarar.
- **Chaves de acesso:** uma por consumidor, com permissão por saída.
- **Navegação por links:** se a saída apontar para um objeto, a API percorre relações, como `/api/clientes/42/pedidos`.

Por padrão uma saída só aceita leitura. A escrita é habilitada por saída, como descrito em Escrita pela API.

Além das saídas, o usuário interage com os dados de outras quatro formas:

- **Explorar:** editor SQL para o técnico; busca, filtros e navegação por links para o analista.
- **Analisar:** gráficos simples na própria ferramenta. Para o resto, Power BI, Metabase e Excel se conectam aos dados unificados.
- **Manipular:** transformar (modelo novo), corrigir (dizer que dois registros são o mesmo cliente) e editar (ações). Por padrão a fonte original não é alterada; edições ficam numa camada por cima, com autor e data.
- **Construir em cima:** aplicações próprias pela API, alertas e um servidor MCP para assistentes de IA consultarem a ontologia.

### Escrita pela API

Uma saída pode aceitar os quatro métodos do REST, e o usuário escolhe onde as alterações ficam guardadas. Sem a chave `methods`, a saída continua só com `GET`.

```yaml
methods: [get, post, patch, delete]
write:
  to: destination       # internal, destination, source_api ou source_database
  conflict: source_wins # source_wins, edit_wins ou latest_wins
```

- `GET`: consulta a lista ou um registro.
- `POST`: cria um registro.
- `PUT` e `PATCH`: alteram um registro, inteiro ou em parte.
- `DELETE`: apaga um registro.

| Destino | Onde a alteração fica | Observação |
| --- | --- | --- |
| `internal` | Dentro da ferramenta, numa camada por cima do dado original | A origem não é tocada |
| `destination` | No banco de destino, junto com os dados unificados | O destino vira o banco de trabalho do workflow |
| `source_api` | Repassada à API do sistema de origem | Respeita as regras de negócio do sistema |
| `source_database` | Direto nas tabelas do sistema de origem | Pula as regras do sistema; só em fontes graváveis, com aviso |

No banco de destino, cada saída gravável tem três partes, para as alterações sobreviverem às atualizações:

- `pedidos_base`: a tabela que vem das fontes, substituída a cada atualização.
- `pedidos_edits`: as alterações feitas pela API, com autor e data. A atualização não toca nela.
- `pedidos`: a visão que combina as duas e mostra o dado final. É o que workflows e ferramentas de BI consultam.

A regra de conflito padrão é a origem vence: uma alteração vale enquanto a origem não mudar aquele campo. Ao gravar, a ferramenta guarda o valor que a origem tinha, e a cada atualização compara.

| Situação | Resultado |
| --- | --- |
| A origem continua com o mesmo valor | A alteração segue valendo |
| A origem mudou o mesmo campo | O valor da origem assume; a alteração vai para o histórico |
| A origem mudou outro campo do registro | As duas mudanças convivem |
| Registro criado pela API | Permanece, até a origem ter um registro com a mesma chave |
| Registro apagado pela API | Fica oculto, até a origem alterar esse registro |

Nada se perde: a alteração superada fica no histórico, marcada com data, e o conflito é sinalizado. As regras `edit_wins` e `latest_wins` podem ser escolhidas por saída.

A escrita tem quatro limites:

- **Só com chave.** Vale para objetos da ontologia ou modelos com chave declarada.
- **Não em dados agregados.** Não se altera um total, só os registros que o compõem.
- **Validação pelos tipos** das propriedades, antes de aceitar.
- **Sempre pela API.** Ela valida e registra autor, data e valor anterior. Gravação direta no banco de destino por SQL não é validada nem registrada.

## Implantação: dois serviços na mesma máquina

O produto roda numa única máquina da empresa como dois processos saídos do mesmo binário. A aplicação web inicia e supervisiona a API de dados.

| Serviço | O que faz | Quem acessa |
| --- | --- | --- |
| Aplicação web | Conectar fontes, modelar, definir ontologia e saídas; agenda as atualizações | Pessoas, pelo navegador, com login |
| API de dados | Serve apenas as saídas publicadas, em outra porta; escrita só onde a saída habilitar | Workflows e sistemas, com chave de acesso |

A separação traz isolamento (uma consulta pesada de modelagem não atrasa a API), segurança (a API pode ser exposta sem expor a administração) e estabilidade (a aplicação reinicia sem derrubar a API).

A ligação entre os dois é a publicação por instantâneo. O DuckDB não permite um processo gravando e outro lendo o mesmo arquivo, então a cada atualização a aplicação gera uma cópia pronta da saída e a troca de uma vez. A API nunca mostra dado pela metade, e dá para voltar ao instantâneo anterior.

A instalação pede três coisas: o binário, a pasta do projeto e uma pasta de dados. As credenciais ficam em variáveis de ambiente. A máquina precisa alcançar os bancos de origem pela rede, então o natural é ficar na rede interna.

O trabalho chega ao servidor pelo Git: o usuário modela, envia a pasta do projeto e o servidor carrega a versão nova. Rodar no computador pessoal continua possível para testar.

Limite assumido: é uma máquina só. O volume de dados é limitado por disco e memória, e não há alta disponibilidade.

## Requisitos do ambiente corporativo

Quatro requisitos valem para todos os conectores, além da lista de fontes:

- **Credenciais fora dos arquivos.** O YAML referencia variáveis de ambiente ou um cofre, nunca a senha.
- **Somente leitura por padrão.** Nenhuma conexão escreve na fonte sem configuração explícita.
- **Sincronização incremental.** Tabelas grandes não são recopiadas inteiras a cada execução.
- **Autenticação corporativa.** Login via Microsoft Entra ou Google, principalmente para SharePoint e Sheets.

## Ambiente de teste e empresa fictícia

Como não há fontes reais disponíveis, o projeto cria uma empresa fictícia com dados sintéticos espalhados por várias fontes, tudo local em Docker. Os dados são gerados com semente fixa, para serem sempre iguais.

| Dado | Onde fica | Papel |
| --- | --- | --- |
| Clientes | Postgres | O "CRM" |
| Pedidos e produtos | SQL Server | O "ERP" |
| Metas de vendas | Planilha Excel | Planejamento comercial |
| Entregas | API REST falsa, escrita em Go | Sistema de logística |
| Histórico antigo | Parquet no MinIO | Arquivo em S3 |

Os dados trazem defeitos de propósito: o mesmo cliente com grafias diferentes em dois sistemas, chaves que não batem e datas em formatos distintos. É o que testa se a unificação e a ontologia funcionam.

O mesmo conjunto serve a três usos: testes de integração automáticos, tutorial de primeiros passos e demonstração. A meta é um comando `demo up` que sobe tudo e permite unir CRM, ERP e planilha em cinco minutos.

Ficam de fora por enquanto SharePoint, Google Sheets e Salesforce, que dependem de contas reais, e Oracle, cujo contêiner é pesado.

## Roteiro

São sete etapas, e cada uma entrega algo usável sozinha. A API para workflows, o caso de uso central, chega na etapa 3, e a escrita na etapa 4.

1. **Unificar.** Aplicação web para conectar fontes, ver o catálogo, explorar dados, consultar com SQL e baixar arquivos. Join entre um Postgres e um Excel. Inclui o ambiente de demonstração e os comandos básicos da linha de comando.
2. **Modelos.** Construtor visual em quadro e lista, modelos em SQL puro, execução em ordem de dependência, linhagem e sincronização de APIs.
3. **Saídas.** API de dados em processo separado, modos cache e ao vivo, planilha por URL, chaves de acesso e documentação OpenAPI. O contrato já prevê os quatro métodos, com a escrita desligada.
4. **Escrita.** Banco de destino, escrita em `internal` e `destination`, regra de conflito e histórico de alterações.
5. **Ontologia.** Compilador de YAML para views, API de objetos com navegação por links e explorador de objetos.
6. **Equipe.** Login, exportação agendada, servidor MCP e registro de acessos.
7. **Devolução à origem.** Escrita em `source_api` e `source_database`, ações, correções e permissões finas.

Modelos em Python ficam para depois da etapa 7, como extensão.

As fontes da onda 2 entram em paralelo com as etapas 2 a 4. O sistema de plugins externos é aberto depois que a interface interna de conectores estabilizar.

## Riscos

| Risco | Por que importa | Mitigação |
| --- | --- | --- |
| Joins lentos entre fontes remotas | O modo attach consulta a fonte a cada vez | Cache como padrão e opção de sync por fonte |
| Saídas ao vivo sobrecarregarem as fontes | Um workflow pode chamar a API milhares de vezes | Ao vivo só por escolha explícita; limite de chamadas por chave |
| Um processo gravando por arquivo DuckDB | Aplicação e API não podem compartilhar o mesmo arquivo | Publicação por instantâneo |
| Uma máquina só | Sem alta disponibilidade; volume limitado por disco e memória | Limite assumido e documentado; instantâneos permitem restaurar rápido |
| Alterações perdidas ou em conflito com a origem | O workflow e o sistema de origem podem mudar o mesmo dado | Tabela de alterações separada, regra de conflito por campo e histórico |
| Escrita direta no banco de origem | Pula as regras de negócio do sistema e pode corromper dados | Só em fontes marcadas como graváveis, com aviso; fica na etapa 7 |
| Ações e permissões finas | É a parte mais difícil do Foundry | Ficam na etapa 7, mas o formato YAML reserva espaço desde o início |
| Formato de arquivo mudar depois de adotado | Quebra projetos de usuários | Versionar o formato desde a primeira versão |
| Compilação cruzada com cgo | Complica a publicação de binários | Automatizar o build por plataforma desde a etapa 1 |
| Extensão ODBC imatura | Pode falhar com bancos corporativos | Conectores próprios em Go para SQL Server e Oracle |
| Escopo crescer demais | Os concorrentes tentam replicar o Foundry inteiro | Manter o recorte leve, numa máquina só, como critério de decisão |

## Decisões em aberto

- [ ] Nome escolhido: Intársia. Falta confirmar a organização no GitHub, os domínios e o registro de marca, com atenção ao CRM Intarsia da SEDONA, dos anos 2000
- [ ] Licença escolhida: Apache 2.0. Falta uma revisão jurídica antes de publicar
- [ ] Campos exatos de cada passo do modelo visual e o SQL que cada um gera
- [ ] Campos de conexão de cada tipo de conector
- [ ] Bibliotecas do frontend e suas versões, na montagem do projeto
- [ ] Assinatura final da interface `Connector`, incluindo sincronização incremental
- [ ] Formato do instantâneo publicado e quantas versões anteriores guardar
- [ ] Como a API de dados acessa as fontes nas saídas ao vivo
- [ ] Login da aplicação web: usuários próprios, Microsoft Entra ou Google
- [ ] Onde guardar credenciais: variáveis de ambiente, chaveiro do sistema ou cofre externo

Já estão decididos a estrutura da pasta, os tipos de modelo, o modelo visual, os campos de fontes, objetos e saídas e as regras de escrita. O nome é Intársia e a licença é Apache 2.0. O repositório único e a arquitetura do código também estão definidos. A arquitetura do frontend também. Com isso o plano cobre o necessário para começar a etapa 1, e os itens acima se resolvem durante a construção.
