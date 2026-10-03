import React from 'react';
import { View } from 'react-native';
import { Button, Card, Text, useTheme } from 'react-native-paper';

type Operation = Record<string, unknown>;

type OperationsScreenProps = {
  operations: Operation[];
  loading: boolean;
  canReconcile: boolean;
  onReconcile: (operation: Operation) => void;
};

const display = (value: unknown) => typeof value === 'string' || typeof value === 'number' ? String(value) : '—';
const canReconcileOperation = (operation: Operation) =>
  Array.isArray(operation.allowed_actions) && operation.allowed_actions.includes('RECONCILE');

export function OperationsScreen({ operations, loading, canReconcile, onReconcile }: OperationsScreenProps) {
  const theme = useTheme();
  return <View testID="operations.screen" style={{ gap: 12, paddingBottom: 16 }}>
    {operations.map(operation => {
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
          {canReconcile && reconcile ? <Button
            testID={`operation-reconcile-${display(operation.id)}`}
            mode="contained-tonal"
            contentStyle={{ minHeight: 48 }}
            accessibilityLabel={`Reconcile операция ${display(operation.id)}`}
            disabled={loading}
            onPress={() => onReconcile(operation)}
          >Reconcile</Button> : null}
        </Card.Content>
      </Card>;
    })}
  </View>;
}
