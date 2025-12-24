```
          █████            █████
          ░░███            ░░███
 ████████  ░███  ████████   ░███ █████     ██████   ██████  █████████████   ████████  ████████   ██████   █████   █████   ██████  ████████
░░███░░███ ░███ ░░███░░███  ░███░░███     ███░░███ ███░░███░░███░░███░░███ ░░███░░███░░███░░███ ███░░███ ███░░   ███░░   ███░░███░░███░░███
 ░███ ░███ ░███  ░███ ░███  ░██████░     ░███ ░░░ ░███ ░███ ░███ ░███ ░███  ░███ ░███ ░███ ░░░ ░███████ ░░█████ ░░█████ ░███ ░███ ░███ ░░░
 ░███ ░███ ░███  ░███ ░███  ░███░░███    ░███  ███░███ ░███ ░███ ░███ ░███  ░███ ░███ ░███     ░███░░░   ░░░░███ ░░░░███░███ ░███ ░███
 ░███████  █████ ████ █████ ████ █████   ░░██████ ░░██████  █████░███ █████ ░███████  █████    ░░██████  ██████  ██████ ░░██████  █████
 ░███░░░  ░░░░░ ░░░░ ░░░░░ ░░░░ ░░░░░     ░░░░░░   ░░░░░░  ░░░░░ ░░░ ░░░░░  ░███░░░  ░░░░░      ░░░░░░  ░░░░░░  ░░░░░░   ░░░░░░  ░░░░░
 ░███                                                                       ░███
 █████                                                                      █████
░░░░░                                                                      ░░░░░
```

## Visão Geral

pInk compressor é um script que converte arquivos de imagem para webp e, se possível, comprime elas. 

## Funcionalidades

- Conversão em lote para WebP
- Processamento paralelo multi-thread
- Controle de qualidade (1-100%)
- Preview antes da execução
- Relatório de redução de tamanho

## Requisitos

Para utilizar este script você precisa do FFmpeg instalado. [Como instalar o FFmpeg?](https://github.com/oop7/ffmpeg-install-guide)

## Como usar
### Execução Básica

```
go run compressor.go ./meu_diretorio
```

### Execução com parâmetros

```
go run compressor.go ./minha_pasta -quality=75 -skip
```

## Parâmetros

| Flag | Atalho | Descrição | Padrão |
|------|--------|-----------|--------|
| `-quality` | `-q` | Qualidade (1-100) | 80 |
| `-skip` | `-s` | Pular preview | false |

## Estrutura de Saída
```
pasta_origem/
├── imagem1.jpg
├── imagem2.png
└── compressed/          ← Pasta criada com as imagens comprimidas
    ├── imagem1.webp
    └── imagem2.webp
```

## Exemplo Completo
```
$ go run compressor.go ./fotos -s

Encontradas 42 imagens para processar
Qualidade: 80%

[✔] foto1.jpg (4.2 MB) → foto1.webp (1.1 MB) [73.8% redução]
[✔] foto2.png (8.7 MB) → foto2.webp (2.3 MB) [73.6% redução]

[STATS] Tamanho original: 125.4 MB → Convertido: 32.1 MB (74.4% redução)
```

## Observações
1. Verifique se o FFmpeg está instalado: `ffmpeg -version`
2. Imagens com nomes duplicados serão sobrescritas
3. A pasta `compressed` será criada automaticamente após o processamento das imagens
4. Suporta os seguintes tipos de arquivo: PNG, JPEG, JPG, GIF
5. Qualidade <50 pode causar artefatos visíveis:
   - **Blocos pixelizados**: Quadrados aparentes na imagem
   - **Banding**: Faixas de cor ao invés de gradientes suaves
   - **Perda de detalhes**: Texturas e bordas borradas
   - **Halos e manchas**: Distorções em áreas de alto contraste
