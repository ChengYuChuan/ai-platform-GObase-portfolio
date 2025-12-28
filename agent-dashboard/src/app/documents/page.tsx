'use client';

import * as React from 'react';
import { UploadZone, DocumentList } from '@/components/documents';
import { useDocuments } from '@/lib/hooks/useDocuments';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

export default function DocumentsPage() {
  const { documents, isLoading, upload, remove, refresh } = useDocuments();

  const handleUpload = async (files: File[]) => {
    await upload(files);
    refresh();
  };

  const handleDelete = async (id: string) => {
    await remove(id);
    refresh();
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Documents</h1>
        <p className="text-muted-foreground">
          Upload and manage documents for RAG-powered question answering.
        </p>
      </div>

      <Tabs defaultValue="all" className="space-y-4">
        <TabsList>
          <TabsTrigger value="all">All Documents</TabsTrigger>
          <TabsTrigger value="upload">Upload</TabsTrigger>
        </TabsList>

        <TabsContent value="all" className="space-y-4">
          <DocumentList
            documents={documents}
            isLoading={isLoading}
            onDelete={handleDelete}
          />
        </TabsContent>

        <TabsContent value="upload" className="space-y-4">
          <UploadZone onUpload={handleUpload} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
