import { AppButton } from '../ui/app-button';
import React from 'react';
import { useCurrentAction } from '../ui/use-current-action';
import { LoadingState, ErrorState } from '../ui/states';
import { foundationCopy } from '../../../../BeeloyBackend/mobile/shared-ui/src/foundation-copy.generated';
import { useUiPreferences } from '../ui/preferences';
import { View } from 'react-native';
import { Card, Text, useTheme } from 'react-native-paper';

type Operation = Record<string, unknown>;

type OperationsScreenProps = {
  operations: Operation[];
  loading: boolean;
  canReconcile: boolean;
  loadError: string;
  onRetry: () => void;
  onReconcile: (operation: Operation) => void;
};

const display = (value: unknown) => typeof value === 'string' || typeof value === 'number' ? String(value) : '—';
const canReconcileOperation = (operation: Operation) =>
  Array.isArray(operation.allowed_actions) && operation.allowed_actions.includes('RECONCILE');

export function OperationsScreen({ operations, loading, canReconcile, onReconcile, loadError, onRetry }: OperationsScreenProps) {
  const theme = useTheme(), admit = useCurrentAction(), { language } = useUiPreferences();
  const valid = operations.every(operation => operation && typeof operation.id === 'string' && !!operation.id.trim()
    && (operation.type === undefined || typeof operation.type === 'string') && (operation.state === undefined || typeof operation.state === 'string')
    && (operation.allowed_actions === undefined || (Array.isArray(operation.allowed_actions) && operation.allowed_actions.every(action => typeof action === 'string'))))
    && new Set(operations.map(operation => operation.id)).size === operations.length;
  const error = loadError || (!valid ? foundationCopy[language].error : '');
  return <View testID="operations.screen" style={{ gap: 12, paddingBottom: 16 }}>
    {loading ? <View testID="operations.loading"><LoadingState /></View> : null}
    {!loading && error ? <View testID="operations.error"><ErrorState message={error} onRetry={() => { if (!loading && admit()) onRetry(); }} /></View> : null}
    {!loading && !error && operations.length === 0 ? <Text testID="operations.empty" variant="bodyMedium">Няма операции за избрания филтър.</Text> : null}
    {!loading && !error && operations.map(operation => {
      const allowed = Array.isArray(operation.allowed_actions) ? operation.allowed_actions : [];
      const reconcile = canReconcileOperation(operation);
      return <Card
        key={display(operation.id)}
        mode="contained" style={{ borderWidth: 1, borderColor: theme.colors.outline, backgroundColor: theme.colors.surface }}
        testID={String(operation.state).includes('UNKNOWN') && reconcile ? 'operation-unknown' : undefined}
      >
        <Card.Content style={{ gap: 8, paddingVertical: 16 }}>
          <Text variant="titleMedium">{display(operation.type)} • {display(operation.state)}</Text>
          <Text variant="bodyMedium">{display(operation.id)} • УНП {display(operation.unp)} • {display(operation.created_at)}</Text>
          <Text variant="bodyMedium">Разрешени действия: {allowed.length ? allowed.map(display).join(', ') : 'няма'}</Text>
          {canReconcile && reconcile ? <AppButton
            testID={`operation-reconcile-${display(operation.id)}`}
            mode="contained-tonal"
            contentStyle={{ minHeight: 48 }}
            accessibilityLabel={`Reconcile операция ${display(operation.id)}`}
            disabled={loading}
            onPress={() => { if (!loading && !error && canReconcile && canReconcileOperation(operation) && operations.includes(operation) && admit()) onReconcile(operation); }}
          >Reconcile</AppButton> : null}
        </Card.Content>
      </Card>;
    })}
  </View>;
}
