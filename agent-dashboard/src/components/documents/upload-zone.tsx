'use client';

import * as React from 'react';
import { Upload, File, X, CheckCircle, AlertCircle, Loader2 } from 'lucide-react';
import { cn, formatBytes } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Card } from '@/components/ui/card';

interface UploadFile {
  id: string;
  file: File;
  progress: number;
  status: 'pending' | 'uploading' | 'success' | 'error';
  error?: string;
}

interface UploadZoneProps {
  onUpload: (files: File[]) => Promise<void>;
  accept?: string;
  maxSize?: number; // in bytes
  maxFiles?: number;
  disabled?: boolean;
}

export function UploadZone({
  onUpload,
  accept = '.pdf,.txt,.md,.doc,.docx',
  maxSize = 10 * 1024 * 1024, // 10MB
  maxFiles = 10,
  disabled = false,
}: UploadZoneProps) {
  const [isDragging, setIsDragging] = React.useState(false);
  const [uploadQueue, setUploadQueue] = React.useState<UploadFile[]>([]);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    if (!disabled) {
      setIsDragging(true);
    }
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const validateFile = (file: File): string | null => {
    if (file.size > maxSize) {
      return `File size exceeds ${formatBytes(maxSize)} limit`;
    }
    const acceptedTypes = accept.split(',').map((t) => t.trim().toLowerCase());
    const fileExt = `.${file.name.split('.').pop()?.toLowerCase()}`;
    if (!acceptedTypes.some((t) => t === fileExt || t === file.type)) {
      return 'File type not supported';
    }
    return null;
  };

  const processFiles = async (files: FileList | File[]) => {
    const fileArray = Array.from(files).slice(0, maxFiles);

    const uploadFiles: UploadFile[] = fileArray.map((file) => ({
      id: Math.random().toString(36).substr(2, 9),
      file,
      progress: 0,
      status: 'pending' as const,
      error: validateFile(file) || undefined,
    }));

    // Mark invalid files as error
    uploadFiles.forEach((uf) => {
      if (uf.error) {
        uf.status = 'error';
      }
    });

    setUploadQueue((prev) => [...prev, ...uploadFiles]);

    // Upload valid files
    const validFiles = uploadFiles.filter((uf) => uf.status === 'pending');
    if (validFiles.length === 0) return;

    for (const uploadFile of validFiles) {
      setUploadQueue((prev) =>
        prev.map((uf) =>
          uf.id === uploadFile.id ? { ...uf, status: 'uploading' } : uf
        )
      );

      try {
        // Simulate progress updates
        for (let progress = 0; progress <= 100; progress += 20) {
          await new Promise((resolve) => setTimeout(resolve, 100));
          setUploadQueue((prev) =>
            prev.map((uf) =>
              uf.id === uploadFile.id ? { ...uf, progress } : uf
            )
          );
        }

        await onUpload([uploadFile.file]);

        setUploadQueue((prev) =>
          prev.map((uf) =>
            uf.id === uploadFile.id
              ? { ...uf, status: 'success', progress: 100 }
              : uf
          )
        );
      } catch (error) {
        setUploadQueue((prev) =>
          prev.map((uf) =>
            uf.id === uploadFile.id
              ? {
                  ...uf,
                  status: 'error',
                  error: error instanceof Error ? error.message : 'Upload failed',
                }
              : uf
          )
        );
      }
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (!disabled && e.dataTransfer.files.length > 0) {
      processFiles(e.dataTransfer.files);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      processFiles(e.target.files);
      e.target.value = '';
    }
  };

  const removeFromQueue = (id: string) => {
    setUploadQueue((prev) => prev.filter((uf) => uf.id !== id));
  };

  const clearCompleted = () => {
    setUploadQueue((prev) =>
      prev.filter((uf) => uf.status !== 'success' && uf.status !== 'error')
    );
  };

  return (
    <div className="space-y-4">
      {/* Drop Zone */}
      <Card
        className={cn(
          'relative border-2 border-dashed p-8 text-center transition-colors',
          isDragging && 'border-primary bg-primary/5',
          disabled && 'cursor-not-allowed opacity-50'
        )}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        <input
          ref={fileInputRef}
          type="file"
          accept={accept}
          multiple
          disabled={disabled}
          onChange={handleFileSelect}
          className="hidden"
        />

        <div className="flex flex-col items-center gap-4">
          <div className="rounded-full bg-primary/10 p-4">
            <Upload className="h-8 w-8 text-primary" />
          </div>
          <div>
            <p className="text-lg font-medium">
              Drag and drop files here
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              or click to browse
            </p>
          </div>
          <Button
            variant="outline"
            disabled={disabled}
            onClick={() => fileInputRef.current?.click()}
          >
            Select Files
          </Button>
          <p className="text-xs text-muted-foreground">
            Supported formats: {accept} (max {formatBytes(maxSize)})
          </p>
        </div>
      </Card>

      {/* Upload Queue */}
      {uploadQueue.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium">Upload Queue</h4>
            <Button
              variant="ghost"
              size="sm"
              onClick={clearCompleted}
              className="text-xs"
            >
              Clear completed
            </Button>
          </div>

          <div className="space-y-2">
            {uploadQueue.map((uploadFile) => (
              <div
                key={uploadFile.id}
                className="flex items-center gap-3 rounded-lg border bg-card p-3"
              >
                <File className="h-5 w-5 shrink-0 text-muted-foreground" />

                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">
                    {uploadFile.file.name}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {formatBytes(uploadFile.file.size)}
                  </p>

                  {uploadFile.status === 'uploading' && (
                    <Progress value={uploadFile.progress} className="mt-2 h-1" />
                  )}

                  {uploadFile.error && (
                    <p className="mt-1 text-xs text-destructive">
                      {uploadFile.error}
                    </p>
                  )}
                </div>

                <div className="shrink-0">
                  {uploadFile.status === 'uploading' && (
                    <Loader2 className="h-5 w-5 animate-spin text-primary" />
                  )}
                  {uploadFile.status === 'success' && (
                    <CheckCircle className="h-5 w-5 text-green-500" />
                  )}
                  {uploadFile.status === 'error' && (
                    <AlertCircle className="h-5 w-5 text-destructive" />
                  )}
                  {uploadFile.status === 'pending' && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8"
                      onClick={() => removeFromQueue(uploadFile.id)}
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
