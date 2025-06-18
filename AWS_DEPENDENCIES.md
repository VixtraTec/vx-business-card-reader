# AWS SDK Dependencies - Versões Separadas

Este documento explica como usamos **versões diferentes** do AWS SDK para resolver problemas de compatibilidade.

## Problema Resolvido

**Erro**: `Failed to save business card to DynamoDB: not found, ResolveEndpointV2`

**Causa**: Incompatibilidades entre versões do AWS SDK v2 para diferentes serviços.

**Solução**: Usar **AWS SDK v1** para S3 e **AWS SDK v2** para DynamoDB.

## Versões Utilizadas

### **AWS SDK v2 (DynamoDB)**
- `github.com/aws/aws-sdk-go-v2` v1.36.5
- `github.com/aws/aws-sdk-go-v2/config` v1.18.45
- `github.com/aws/aws-sdk-go-v2/service/dynamodb` v1.22.2
- `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue` v1.10.42

### **AWS SDK v1 (S3)**
- `github.com/aws/aws-sdk-go` v1.44.327

## Por que Versões Separadas?

### ✅ **Vantagens**
- **S3 SDK v1** é **comprovadamente estável** para upload/download
- **DynamoDB SDK v2** tem **recursos modernos** (attributevalue)
- **Sem conflitos** de ResolveEndpointV2
- **Compatibilidade garantida**

### ⚠️ **Trade-offs**
- Duas dependências do AWS SDK
- Sintaxes ligeiramente diferentes
- Tamanho binário um pouco maior

## Estrutura Implementada

```
┌─────────────────┐    AWS SDK v1    ┌─────────────────┐
│   S3 Service    │ ←──────────────→ │       S3        │
└─────────────────┘                  └─────────────────┘

┌─────────────────┐    AWS SDK v2    ┌─────────────────┐
│ DynamoDB Service│ ←──────────────→ │    DynamoDB     │
└─────────────────┘                  └─────────────────┘
```

## Logs Melhorados

Agora os logs incluem `sdk_version` para identificar qual SDK está sendo usado:

```json
{
  "level": "info",
  "operation": "S3UploadImage",
  "message": "Starting S3 upload",
  "file_name": "business-card.jpg",
  "bucket": "vx-src-api-test",
  "sdk_version": "v1"
}
```

## Configurações Necessárias

```bash
# S3 Configuration (SDK v1)
S3_BUCKET_NAME=vx-src-api-test
S3_REGION=us-east-1

# DynamoDB Configuration (SDK v2)
AWS_REGION=us-east-1
DYNAMODB_TABLE_NAME=business-card-reader

# Credenciais (compartilhadas)
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret
```

## Estrutura de Response

```json
{
  "images": [
    {
      "file_name": "business-card.jpg",
      "content_type": "image/jpeg",
      "size": 231215,
      "s3_key": "business-cards/2025/01/15/uuid.jpg", 
      "s3_url": "https://vx-src-api-test.s3.us-east-1.amazonaws.com/business-cards/2025/01/15/uuid.jpg",
      "uploaded_at": "2025-01-15T10:30:00Z"
    }
  ]
}
```

## Benefícios da Solução

### **Para S3 (SDK v1)**
- ✅ **Estabilidade comprovada** para upload/download
- ✅ **Sem problemas de endpoint** resolution
- ✅ **Sintaxe simples** e direta
- ✅ **Performance otimizada** para operações de arquivo

### **Para DynamoDB (SDK v2)**
- ✅ **Recursos modernos** (attributevalue marshaling)
- ✅ **Type safety** melhorada
- ✅ **Context support** nativo
- ✅ **Performance** otimizada para NoSQL

## Troubleshooting

### Se S3 der erro de credenciais:
```bash
# Verificar se bucket existe e tem permissões
aws s3 ls s3://vx-src-api-test --region us-east-1

# Testar upload manual
aws s3 cp test.jpg s3://vx-src-api-test/test.jpg
```

### Se DynamoDB der erro:
```bash
# Verificar se tabela existe
aws dynamodb describe-table --table-name business-card-reader --region us-east-1

# Testar item simples
aws dynamodb put-item --table-name business-card-reader --item '{"id":{"S":"test"}}'
```

### Verificar logs por SDK:
```bash
# Ver apenas operações S3 (SDK v1)
cat logs.json | jq 'select(.sdk_version == "v1")'

# Ver apenas operações DynamoDB (SDK v2)
cat logs.json | jq 'select(.operation | contains("Dynamo"))'
```

## Comandos de Manutenção

### Atualizar apenas S3 SDK v1:
```bash
go get github.com/aws/aws-sdk-go@v1.44.327
go mod tidy
```

### Atualizar apenas DynamoDB SDK v2:
```bash
go get github.com/aws/aws-sdk-go-v2/service/dynamodb@v1.22.2
go mod tidy
```

### Em caso de problemas, resetar:
```bash
go clean -modcache
rm go.sum
go mod tidy
go build
```

## Status da Solução

- ✅ **S3 Upload/Download** funcionando com SDK v1
- ✅ **DynamoDB Save/Read** funcionando com SDK v2  
- ✅ **Logs estruturados** com identificação de SDK
- ✅ **Erro ResolveEndpointV2** resolvido
- ✅ **Compatibilidade** garantida para ambos

Esta abordagem **híbrida** resolve definitivamente os problemas de compatibilidade! 🎉 