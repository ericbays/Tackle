import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { engineeringApi } from '../services/engineeringApi';
import { Server, Activity, Power, RefreshCw, Trash2 } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { Card } from '../../../components/ui/Card';
import { Table, TableHeader, TableRow, TableHead, TableBody, TableCell } from '../../../components/ui/Table';
import { Button } from '../../../components/ui/Button';

export default function InfrastructureTab() {
  const { data: endpoints, isLoading } = useQuery({
    queryKey: ['endpoints'],
    queryFn: engineeringApi.getEndpoints,
  });

  const getStatusVariant = (status: string, health: string) => {
    if (status === 'error') return 'destructive';
    if (status === 'provisioning') return 'secondary';
    if (status === 'stopped') return 'secondary';
    if (health === 'unhealthy') return 'warning';
    return 'success'; // running & healthy
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-2 text-lg font-semibold text-slate-200">
          <Server size={20} className="text-blue-400" /> Active Infrastructure (Endpoints)
        </div>
      </div>
      
      <Card className="rounded-lg overflow-hidden p-0 border-slate-800 shadow-none bg-[#12182b]">
        <div className="overflow-x-auto">
          <Table className="w-full text-left text-sm whitespace-nowrap border-0 shadow-none">
            <TableHeader className="bg-[#1a2235]">
              <TableRow className="text-slate-300 font-medium border-b border-slate-800 hover:bg-transparent">
                <TableHead>Instance ID</TableHead>
                <TableHead>Campaign</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Public IP</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-slate-800/50">
              {isLoading ? (
                <TableRow><TableCell colSpan={6} className="py-8 text-center text-slate-500">Loading...</TableCell></TableRow>
              ) : endpoints?.length === 0 ? (
                <TableRow><TableCell colSpan={6} className="py-12 text-center text-slate-500">No active endpoints found for any campaigns.</TableCell></TableRow>
              ) : (
                endpoints?.map((ep) => (
                  <TableRow key={ep.id} className="hover:bg-slate-800/30 transition-colors">
                    <TableCell className="text-slate-200 font-mono text-xs">{ep.instance_id || 'Pending'}</TableCell>
                    <TableCell className="text-slate-300">{ep.campaign_id}</TableCell>
                    <TableCell className="text-slate-400 capitalize">{ep.cloud_provider}</TableCell>
                    <TableCell className="text-slate-300 font-mono text-xs">{ep.public_ip || '---.---.---.---'}</TableCell>
                    <TableCell>
                      <Badge variant={getStatusVariant(ep.status, ep.health_status) as any} className="gap-1.5 capitalize">
                        {ep.status === 'running' && ep.health_status === 'healthy' ? <Activity size={12}/> : <span className="w-1.5 h-1.5 rounded-full bg-current"></span>}
                        {ep.status === 'running' ? ep.health_status : ep.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2 text-slate-400">
                        <Button variant="ghost" size="icon" title="Restart">
                          <RefreshCw size={14} className="text-amber-400" />
                        </Button>
                        <Button variant="ghost" size="icon" title="Stop">
                          <Power size={14} className="text-slate-200" />
                        </Button>
                        <Button variant="ghost" size="icon" title="Terminate" className="hover:bg-red-500/10 hover:text-red-400">
                          <Trash2 size={14} className="text-red-400" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </Card>
    </div>
  );
}
