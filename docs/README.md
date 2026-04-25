# Documentação do Projeto

Este diretório reúne a documentação oficial do projeto em camadas. A [README raiz](../README.md) funciona como página de visão geral; esta página funciona como portal da documentação técnica.

> ⚠️ **IMPORTANTE:** Esta documentação é baseada em **cenários reais com 2+ máquinas** (rede 172.16.201.0/24). Use scripts em `arquivos_sh/` para reproduzir os testes. Não há exemplo "local" neste v2.0.

## Leitura recomendada

### 📋 Para validação PRÁTICA (comece aqui!)
1. **[TESTING_GUIDE_v2.md](TESTING_GUIDE_v2.md)** ⭐ - Cenários reais:
   - Baseline: 2 servidores (172.16.201.5 + .6)
   - Scale: 4 servidores + stress_sensores/atuadores
   - Stress: 4×20 sensores/drones
   - Failover: Validar reconexão

2. **[REFACTORING_SUMMARY.md](REFACTORING_SUMMARY.md)** - Deployment checklist (copy-paste pronto)

### 📚 Para compreensão técnica
3. **[REFACTORING_README.md](REFACTORING_README.md)** - Quick start + "Deploying to Lab Network"
4. **[MODULARIZATION_CHANGELOG.md](MODULARIZATION_CHANGELOG.md)** - Arquitetura e validação prática de cada módulo
5. **[CODE_REVIEW_GUIDE.md](CODE_REVIEW_GUIDE.md)** - Linha por linha + "During Stress Testing"

### ✅ Para consolidação
6. **[FINAL_REPORT.md](FINAL_REPORT.md)** - Resultados validados de todos 4 cenários
7. **[FILE_INDEX.txt](FILE_INDEX.txt)** - Mapa rápido dos arquivos

## Organização por tema

### ⚡ RÁPIDO - Para Começar Hoje (30 min)
- [REFACTORING_SUMMARY.md](REFACTORING_SUMMARY.md) - Deployment checklist
- [TESTING_GUIDE_v2.md - Scenario 1](TESTING_GUIDE_v2.md#1️⃣-cenário-baseline-2-servidores--1-sensor--1-drone) - 2-server baseline

### 🏗️ APROFUNDADO - Para Entender Arquitetura (2h)
- [MODULARIZATION_CHANGELOG.md](MODULARIZATION_CHANGELOG.md) - Cada módulo explicado
- [CODE_REVIEW_GUIDE.md](CODE_REVIEW_GUIDE.md) - Detalhes de implementação

### 🧪 PRÁTICO - Para Testar em Lab (4h)
- [TESTING_GUIDE_v2.md](TESTING_GUIDE_v2.md) - 4 cenários progressivos:
  - Scenario 1: 2-server (30 min)
  - Scenario 2: 4-server (45 min)  
  - Scenario 3: Stress (60 min)
  - Scenario 4: Failover (30 min)

### 📖 REFERÊNCIA - Para Consultas Rápidas
- [FILE_INDEX.txt](FILE_INDEX.txt) - Mapa de todos arquivos

## Convenções desta documentação

- A README raiz apresenta a solução em alto nível.
- Este diretório guarda as páginas temáticas e os relatórios.
- Cada documento aqui tem um propósito único para facilitar navegação, revisão e manutenção.
- Links internos devem apontar para arquivos deste diretório quando o assunto for documentação detalhada.

## Caminho sugerido para novos leitores

### 🚀 Quero fazer o deploy agora! (30 min)
1. Ler [REFACTORING_SUMMARY.md - Deployment Checklist](REFACTORING_SUMMARY.md#-deployment-checklist-lab-network-172162010024)
2. Rodar [TESTING_GUIDE_v2.md - Scenario 1](TESTING_GUIDE_v2.md#1️⃣-cenário-baseline-2-servidores--1-sensor--1-drone) (2-server baseline)
3. Validar com [FINAL_REPORT.md - Validação no Cenário 1](FINAL_REPORT.md#cenário-1-2-server-baseline)

### 🎓 Quero entender a arquitetura (2-3h)
1. Ler a [README raiz](../README.md) para contexto
2. Estudar [REFACTORING_README.md](REFACTORING_README.md) seção "🌐 Deploying to Lab Network"
3. Aprofundar em [MODULARIZATION_CHANGELOG.md](MODULARIZATION_CHANGELOG.md) cada módulo
4. Ver o lado prático em [CODE_REVIEW_GUIDE.md - During Stress Testing](CODE_REVIEW_GUIDE.md#🧪-durante-stress-testing-validação-prática)
5. Consolidar em [FINAL_REPORT.md](FINAL_REPORT.md)

### 🧪 Quero rodar todos os cenários (4-5h)
1. Preparar 2+ máquinas na rede 172.16.201.0/24
2. Seguir [TESTING_GUIDE_v2.md](TESTING_GUIDE_v2.md) em ordem:
   - Scenario 1 (Baseline, 2-server): 30 min
   - Scenario 2 (Scale, 4-server): 45 min
   - Scenario 3 (Stress, 4×20): 60 min
   - Scenario 4 (Failover): 30 min
3. Comparar resultados com [FINAL_REPORT.md - Validação](FINAL_REPORT.md#✅-validação-em-cenários-reais-172162010024)
4. Usar [CODE_REVIEW_GUIDE.md - During Stress Testing](CODE_REVIEW_GUIDE.md#🧪-durante-stress-testing-validação-prática) para debug

### 🔍 Quero fazer code review (referência)
1. Seguir [CODE_REVIEW_GUIDE.md](CODE_REVIEW_GUIDE.md) ordem de revisão (types → lamport → ricart → ...)
2. Procurar RED FLAGS durante [CODE_REVIEW_GUIDE.md - Red Flags](CODE_REVIEW_GUIDE.md#-red-flags-o-que-procurar)
3. Validar contra checklist em [CODE_REVIEW_GUIDE.md - Checklist Final](CODE_REVIEW_GUIDE.md#-checklist-final-de-review)
