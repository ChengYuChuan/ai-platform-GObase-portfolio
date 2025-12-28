'use client';

import { useState, useCallback } from 'react';
import useSWR from 'swr';
import type { DocumentUploadProgress } from '@/types';
import { listDocuments, uploadDocument, deleteDocument } from '@/lib/api';
import { generateId } from '@/lib/utils';

/**
 * Hook for managing documents
 */
export function useDocuments() {
  const [uploadProgress, setUploadProgress] = useState<DocumentUploadProgress[]>([]);
  const [error, setError] = useState<string | null>(null);

  const {
    data,
    error: fetchError,
    isLoading,
    mutate,
  } = useSWR('documents', async () => {
    const response = await listDocuments();
    return response.documents;
  });

  const documents = data || [];

  const upload = useCallback(
    async (files: File[]): Promise<void> => {
      const uploadPromises = files.map(async (file) => {
        const uploadId = generateId();

        // Add to progress tracking
        setUploadProgress((prev) => [
          ...prev,
          {
            documentId: uploadId,
            progress: 0,
            status: 'pending',
          },
        ]);

        try {
          // Update status to processing
          setUploadProgress((prev) =>
            prev.map((p) =>
              p.documentId === uploadId
                ? { ...p, status: 'processing', progress: 50 }
                : p
            )
          );

          // Upload document
          const doc = await uploadDocument(file);

          // Update status to complete
          setUploadProgress((prev) =>
            prev.map((p) =>
              p.documentId === uploadId
                ? { ...p, documentId: doc.id, status: 'ready' as const, progress: 100 }
                : p
            )
          );

          return doc;
        } catch (err) {
          const errorMessage = err instanceof Error ? err.message : 'Upload failed';

          setUploadProgress((prev) =>
            prev.map((p) =>
              p.documentId === uploadId
                ? { ...p, status: 'error', error: errorMessage }
                : p
            )
          );

          throw err;
        }
      });

      try {
        await Promise.all(uploadPromises);
        // Refresh document list
        mutate();
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Upload failed');
      } finally {
        // Clear completed uploads after a delay
        setTimeout(() => {
          setUploadProgress((prev) =>
            prev.filter((p) => p.status !== 'ready')
          );
        }, 3000);
      }
    },
    [mutate]
  );

  const remove = useCallback(
    async (documentId: string): Promise<void> => {
      try {
        await deleteDocument(documentId);
        mutate();
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Delete failed');
        throw err;
      }
    },
    [mutate]
  );

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  const refresh = useCallback(() => {
    mutate();
  }, [mutate]);

  return {
    documents,
    isLoading,
    error: error || fetchError?.message,
    uploadProgress,
    upload,
    remove,
    refresh,
    clearError,
  };
}

/**
 * Hook for document search
 */
export function useDocumentSearch(query: string) {
  const { documents } = useDocuments();

  const searchResults = documents.filter((doc) => {
    if (!query) return true;
    const searchLower = query.toLowerCase();
    const filename = doc.filename || doc.name || '';
    const contentType = doc.contentType || doc.type || '';
    return (
      filename.toLowerCase().includes(searchLower) ||
      contentType.toLowerCase().includes(searchLower)
    );
  });

  return {
    results: searchResults,
    isSearching: false,
    total: documents.length,
  };
}
