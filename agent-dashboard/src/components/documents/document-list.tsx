'use client';

import * as React from 'react';
import {
  FileText,
  Trash2,
  Download,
  Eye,
  Search,
  Filter,
  SortAsc,
  SortDesc,
} from 'lucide-react';
import { formatBytes, formatDate } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { Document } from '@/types';

interface DocumentListProps {
  documents: Document[];
  isLoading?: boolean;
  onDelete?: (id: string) => Promise<void>;
  onView?: (document: Document) => void;
  onDownload?: (document: Document) => void;
}

type SortField = 'name' | 'uploadedAt' | 'size';
type SortOrder = 'asc' | 'desc';

const statusColors: Record<Document['status'], string> = {
  pending: 'warning',
  processing: 'default',
  ready: 'success',
  error: 'destructive',
};

export function DocumentList({
  documents,
  isLoading = false,
  onDelete,
  onView,
  onDownload,
}: DocumentListProps) {
  const [searchQuery, setSearchQuery] = React.useState('');
  const [sortField, setSortField] = React.useState<SortField>('uploadedAt');
  const [sortOrder, setSortOrder] = React.useState<SortOrder>('desc');
  const [typeFilter, setTypeFilter] = React.useState<string>('all');
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [documentToDelete, setDocumentToDelete] = React.useState<Document | null>(null);
  const [isDeleting, setIsDeleting] = React.useState(false);

  // Get unique file types for filter
  const fileTypes = React.useMemo(() => {
    const types = new Set(documents.map((d) => d.type));
    return Array.from(types);
  }, [documents]);

  // Filter and sort documents
  const filteredDocuments = React.useMemo(() => {
    let result = [...documents];

    // Search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      result = result.filter(
        (d) =>
          d.name.toLowerCase().includes(query) ||
          d.type.toLowerCase().includes(query)
      );
    }

    // Type filter
    if (typeFilter !== 'all') {
      result = result.filter((d) => d.type === typeFilter);
    }

    // Sort
    result.sort((a, b) => {
      let comparison = 0;
      switch (sortField) {
        case 'name':
          comparison = a.name.localeCompare(b.name);
          break;
        case 'uploadedAt':
          comparison = new Date(a.uploadedAt).getTime() - new Date(b.uploadedAt).getTime();
          break;
        case 'size':
          comparison = a.size - b.size;
          break;
      }
      return sortOrder === 'asc' ? comparison : -comparison;
    });

    return result;
  }, [documents, searchQuery, sortField, sortOrder, typeFilter]);

  const handleDeleteClick = (document: Document) => {
    setDocumentToDelete(document);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!documentToDelete || !onDelete) return;

    setIsDeleting(true);
    try {
      await onDelete(documentToDelete.id);
      setDeleteDialogOpen(false);
      setDocumentToDelete(null);
    } finally {
      setIsDeleting(false);
    }
  };

  const toggleSortOrder = () => {
    setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'));
  };

  if (isLoading) {
    return (
      <Card>
        <CardContent className="flex items-center justify-center py-12">
          <div className="text-center">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent mx-auto" />
            <p className="mt-4 text-sm text-muted-foreground">Loading documents...</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle>Documents ({documents.length})</CardTitle>

            {/* Filters */}
            <div className="flex flex-wrap items-center gap-2">
              {/* Search */}
              <div className="relative">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search documents..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-[200px] pl-9"
                />
              </div>

              {/* Type filter */}
              <Select value={typeFilter} onValueChange={setTypeFilter}>
                <SelectTrigger className="w-[120px]">
                  <Filter className="mr-2 h-4 w-4" />
                  <SelectValue placeholder="Type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All types</SelectItem>
                  {fileTypes.map((type) => (
                    <SelectItem key={type} value={type}>
                      {type.toUpperCase()}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Sort */}
              <Select value={sortField} onValueChange={(v) => setSortField(v as SortField)}>
                <SelectTrigger className="w-[120px]">
                  <SelectValue placeholder="Sort by" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="uploadedAt">Date</SelectItem>
                  <SelectItem value="name">Name</SelectItem>
                  <SelectItem value="size">Size</SelectItem>
                </SelectContent>
              </Select>

              <Button variant="ghost" size="icon" onClick={toggleSortOrder}>
                {sortOrder === 'asc' ? (
                  <SortAsc className="h-4 w-4" />
                ) : (
                  <SortDesc className="h-4 w-4" />
                )}
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent>
          {filteredDocuments.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <FileText className="h-12 w-12 text-muted-foreground/50" />
              <h3 className="mt-4 text-lg font-semibold">No documents found</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                {searchQuery || typeFilter !== 'all'
                  ? 'Try adjusting your search or filters'
                  : 'Upload documents to get started'}
              </p>
            </div>
          ) : (
            <div className="divide-y">
              {filteredDocuments.map((document) => (
                <div
                  key={document.id}
                  className="flex items-center gap-4 py-4 first:pt-0 last:pb-0"
                >
                  {/* Icon */}
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                    <FileText className="h-5 w-5 text-primary" />
                  </div>

                  {/* Info */}
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <p className="truncate font-medium">{document.name}</p>
                      <Badge variant={statusColors[document.status] as 'default'}>
                        {document.status}
                      </Badge>
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {formatBytes(document.size)} • {document.type.toUpperCase()} •{' '}
                      {formatDate(document.uploadedAt)}
                    </p>
                    {document.chunkCount !== undefined && (
                      <p className="text-xs text-muted-foreground">
                        {document.chunkCount} chunks
                      </p>
                    )}
                  </div>

                  {/* Actions */}
                  <div className="flex shrink-0 items-center gap-1">
                    {onView && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => onView(document)}
                      >
                        <Eye className="h-4 w-4" />
                      </Button>
                    )}
                    {onDownload && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => onDownload(document)}
                      >
                        <Download className="h-4 w-4" />
                      </Button>
                    )}
                    {onDelete && (
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleDeleteClick(document)}
                      >
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Document</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete &quot;{documentToDelete?.name}&quot;?
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteDialogOpen(false)}
              disabled={isDeleting}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteConfirm}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
