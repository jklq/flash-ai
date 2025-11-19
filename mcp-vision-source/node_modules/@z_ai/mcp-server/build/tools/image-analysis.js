import { z } from 'zod';
import { FileNotFoundError, ApiError } from '../types/index.js';
import { ToolExecutionError } from '../core/error-handler.js';
import { CommonSchemas, ToolSchemaBuilder } from '../utils/validation.js';
import { createMultiModalMessage, createImageContent, formatMcpResponse, createSuccessResponse, createErrorResponse, withRetry } from '../core/api-common.js';
import { fileService } from '../core/file-service.js';
import { chatService } from '../core/chat-service.js';
/**
 * Image analysis service class
 */
export class ImageAnalysisService {
    chatService = chatService;
    fileService = fileService;
    MAX_IMAGE_SIZE_MB = 5;
    /**
     * Execute image analysis
     * @param request Image analysis request
     * @returns Analysis result
     */
    async analyzeImage(request) {
        console.info('Starting image analysis', {
            imageSource: request.imageSource,
            prompt: request.prompt
        });
        try {
            // Validate image source (file or URL) and size
            await this.fileService.validateImageSource(request.imageSource, this.MAX_IMAGE_SIZE_MB);
            // Validate prompt
            if (!request.prompt || request.prompt.trim().length === 0) {
                throw new ToolExecutionError('Prompt is required for image analysis', 'image-analysis', 'VALIDATION_ERROR', {
                    toolName: 'image-analysis',
                    operation: 'analyzeImage',
                    metadata: { imageSource: request.imageSource }
                });
            }
            // Handle image source (URL or local file)
            let imageContent;
            if (this.fileService.isUrl(request.imageSource)) {
                // For URLs, pass directly without base64 encoding
                imageContent = createImageContent(request.imageSource);
            }
            else {
                // For local files, encode to base64
                const imageData = await this.fileService.encodeImageToBase64(request.imageSource);
                imageContent = createImageContent(imageData);
            }
            // Create multimodal message
            const messages = createMultiModalMessage([imageContent], request.prompt);
            const result = await this.chatService.visionCompletions(messages);
            console.info('Image analysis completed', {
                imageSource: request.imageSource
            });
            return result;
        }
        catch (error) {
            console.error('Image analysis failed', {
                error: error instanceof Error ? error.message : String(error),
                imageSource: request.imageSource
            });
            if (error instanceof ToolExecutionError) {
                throw error;
            }
            // Wrap unknown errors
            throw new ToolExecutionError(`Image analysis failed: ${error.message}`, 'image-analysis', 'EXECUTION_ERROR', {
                toolName: 'image-analysis',
                operation: 'analyzeImage',
                metadata: { imageSource: request.imageSource, originalError: error }
            }, error);
        }
    }
}
/**
 * Register image analysis tool with MCP server
 * @param server MCP server instance
 */
export function registerImageAnalysisTool(server) {
    const analysisService = new ImageAnalysisService();
    const retryableAnalyze = withRetry(analysisService.analyzeImage.bind(analysisService), 2, // Maximum 2 retries
    1000 // 1 second delay
    );
    server.tool('analyze_image', 'Analyze an image using advanced AI vision models with comprehensive understanding capabilities. Supports both local files and remote URL. Maximum file size: 5MB', {
        image_source: z.string().describe('Local file path or remote URL to the image (supports PNG, JPG, JPEG)'),
        prompt: z.string().describe('Detailed text prompt. If the task is **front-end code replication**, the prompt you provide must be: "Describe in detail the layout structure, color style, main components, and interactive elements of the website in this image to facilitate subsequent code generation by the model." + your additional requirements. \ For **other tasks**, the prompt you provide must clearly describe what to analyze, extract, or understand from the image.')
    }, async (params) => {
        try {
            // Validate parameters
            const validationSchema = new ToolSchemaBuilder()
                .required('image_source', CommonSchemas.nonEmptyString)
                .required('prompt', CommonSchemas.nonEmptyString)
                .build();
            validationSchema.parse(params);
            // Build request object
            const request = {
                imageSource: params.image_source,
                prompt: params.prompt
            };
            // Execute analysis
            const result = await retryableAnalyze(request);
            const response = createSuccessResponse(result);
            return formatMcpResponse(response);
        }
        catch (error) {
            console.error('Tool execution failed', {
                error: error instanceof Error ? error.message : String(error),
                params
            });
            let errorResponse;
            if (error instanceof z.ZodError) {
                const validationErrors = error.issues.map((e) => `${e.path.join('.')}: ${e.message}`).join(', ');
                errorResponse = createErrorResponse(`Validation failed: ${validationErrors}`);
            }
            else if (error instanceof FileNotFoundError) {
                errorResponse = createErrorResponse(`Image file not found: ${error.message}`);
            }
            else if (error instanceof ApiError) {
                errorResponse = createErrorResponse(`API error: ${error.message}`);
            }
            else {
                errorResponse = createErrorResponse(`Unexpected error: ${error instanceof Error ? error.message : String(error)}`);
            }
            return formatMcpResponse(errorResponse);
        }
    });
    console.info('Image analysis tool registered successfully');
}
