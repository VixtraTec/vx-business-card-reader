# Sistema de Logging Estruturado

Este documento descreve o sistema de logging estruturado implementado para facilitar o debugging e monitoramento da aplicação.

## Configuração

### Log Level
Configure o nível de log através da variável de ambiente:
```bash
LOG_LEVEL=debug   # Para desenvolvimento e debugging
LOG_LEVEL=info    # Para produção
LOG_LEVEL=warn    # Apenas warnings e erros
LOG_LEVEL=error   # Apenas erros
```

### Formato
Os logs são emitidos em formato JSON estruturado com timestamp:
```json
{
  "level": "info",
  "time": "2025-01-15 10:30:00",
  "operation": "ProcessBusinessCard",
  "message": "Starting business card processing",
  "image_count": 1,
  "content_type": "application/json"
}
```

## Logs Implementados

### 1. Handler (business_card_handler.go)
**ProcessBusinessCard**
- ✅ Início do processamento (IP, User-Agent, Content-Type)
- ✅ Detecção de tipo de requisição (JSON vs Multipart)

**processBusinessCardFromJSON**
- ✅ Recebimento da requisição (content_length)
- ✅ Parse do JSON (image_count, total_images, timestamp)
- ✅ Validações (image_count, size_mismatch)
- ✅ Processamento de cada imagem (file_name, size, content_type)
- ✅ Decodificação base64 (file_name, size)
- ✅ Processamento do business card (image_count)
- ✅ Resultado final (business_card_id, status)

### 2. S3 Service (s3_service.go)
**UploadImage**
- ✅ Início do upload (file_name, content_type, size, s3_key, bucket)
- ✅ Sucesso do upload (file_name, s3_key, s3_url)
- ✅ Erros de upload (file_name, s3_key, bucket)

**GetImage**
- ✅ Início do download (s3_key, bucket)
- ✅ Sucesso do download (s3_key, size)
- ✅ Erros de download (s3_key, bucket, step)

### 3. Business Card Service (business_card_service.go)
**ProcessBusinessCard**
- ✅ Início do processamento (image_count)
- ✅ Processamento de cada imagem (index, file_name, content_type, size)
- ✅ Upload S3 (file_name, s3_key, s3_url)
- ✅ Criação do registro (business_card_id, status)
- ✅ Salvamento no DynamoDB (business_card_id)
- ✅ Processamento Gemini (business_card_id)
- ✅ Estados de erro (business_card_id, error, step)
- ✅ Sucesso final (business_card_id, status)

### 4. DynamoDB Service (dynamo_service.go)
**NewDynamoService**
- ✅ Inicialização (table_name, region)
- ✅ Erros de configuração (region, step)

**SaveBusinessCard**
- ✅ Início do salvamento (business_card_id, status, table_name)
- ✅ Marshal do objeto (business_card_id, item_keys)
- ✅ Put item (business_card_id, table_name)
- ✅ Sucesso (business_card_id)
- ✅ Erros (business_card_id, table_name, step)

## Campos de Log Padronizados

### Identificadores
- `business_card_id`: ID único do business card
- `file_name`: Nome do arquivo de imagem
- `s3_key`: Chave do objeto no S3
- `table_name`: Nome da tabela DynamoDB

### Contexto Técnico
- `step`: Etapa específica onde ocorreu erro/ação
- `operation`: Nome da operação principal
- `content_type`: Tipo de conteúdo da imagem
- `size`: Tamanho em bytes
- `status`: Status do business card

### Contexto de Requisição
- `client_ip`: IP do cliente
- `user_agent`: User agent do cliente
- `content_length`: Tamanho da requisição

## Como Usar para Debugging

### 1. Erro 500 - Ver logs estruturados
```bash
# Filtrar logs por business_card_id específico
cat logs.json | jq 'select(.business_card_id == "uuid-aqui")'

# Ver apenas erros
cat logs.json | jq 'select(.level == "error")'

# Ver fluxo completo de uma operação
cat logs.json | jq 'select(.operation == "ProcessBusinessCard")'
```

### 2. Monitorar S3 uploads
```bash
# Ver uploads do S3
cat logs.json | jq 'select(.operation == "S3UploadImage")'
```

### 3. Monitorar DynamoDB
```bash
# Ver operações DynamoDB
cat logs.json | jq 'select(.operation | contains("Dynamo"))'
```

## Benefícios

### Para Desenvolvimento
- ✅ **Rastreabilidade completa** do fluxo de dados
- ✅ **Context específico** para cada erro
- ✅ **Debugging eficiente** com IDs únicos
- ✅ **Performance monitoring** com sizes e timings

### Para Produção
- ✅ **Logs estruturados** para ferramentas de monitoring
- ✅ **Correlação** entre requisições e operações
- ✅ **Alerting** baseado em campos específicos
- ✅ **Troubleshooting** rápido com contexto completo

## Exemplo de Debug Session

```bash
# 1. Ver erro 500
tail -f logs.json | jq 'select(.level == "error")'

# 2. Pegar business_card_id do erro
business_card_id="abc-123-def"

# 3. Ver fluxo completo
cat logs.json | jq --arg id "$business_card_id" 'select(.business_card_id == $id)'

# 4. Ver apenas steps específicos
cat logs.json | jq --arg id "$business_card_id" 'select(.business_card_id == $id and .step == "s3_upload")'
```

Com este sistema, o debugging de erros 500 ficou muito mais eficiente! 🎉 