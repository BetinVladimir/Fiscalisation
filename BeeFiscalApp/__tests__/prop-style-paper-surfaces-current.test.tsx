// Authored only; global Paper/test-writing gate remains closed.
import React from 'react';
import {act,create} from 'react-test-renderer';
jest.mock('react-native',()=>({Platform:{OS:'web'},View:'Layout',ScrollView:'Scroll',StyleSheet:{create:(x:any)=>x},useWindowDimensions:()=>({width:320,fontScale:1})}));
jest.mock('react-native-paper',()=>({Surface:'Paper.Surface',Text:'Paper.Text',Appbar:{Header:'Header',Content:'Content'},useTheme:()=>({colors:{background:'#fff'}})}));
jest.mock('react-native-safe-area-context',()=>({useSafeAreaInsets:()=>({top:0,bottom:0,left:0,right:0})}));
jest.mock('../src/ui/responsive',()=>({useResponsiveLayout:()=>({gutter:16})}));
import {AuthForm} from '../src/ui/auth-form';
import {Screen} from '../src/ui/screen';
test('auth decoration is owned by Paper while measurement remains technical',async()=>{
 let tree:any;const style={backgroundColor:'#fff',borderWidth:1};
 await act(async()=>{tree=create(<AuthForm testID="auth-form" style={style}>Fields</AuthForm>)});
 const surface=tree.root.findByType('Paper.Surface');expect(surface.props.style[0]).toBe(style);expect(surface.props.mode).toBe('flat');expect(surface.props.elevation).toBe(0);
 const measurement=tree.root.findByType('Layout');await act(async()=>measurement.props.onLayout({nativeEvent:{layout:{width:320}}}));expect(tree.root.findByType('Paper.Surface').props.style[1].width).toBe(320);
 expect(measurement.props.style.backgroundColor).toBeUndefined();await act(async()=>tree.unmount());
});
test.each([true,false])('screen content decoration stays on Paper, scroll=%s',async scroll=>{
 let tree:any;const style={backgroundColor:'#fff',borderWidth:1};
 await act(async()=>{tree=create(<Screen scroll={scroll} contentStyle={style}>Body</Screen>)});
 const surfaces=tree.root.findAllByType('Paper.Surface');expect(surfaces).toHaveLength(2);expect(surfaces[1].props.style).toContain(style);expect(surfaces[1].props.mode).toBe('flat');expect(surfaces[1].props.elevation).toBe(0);expect(tree.root.findAllByType('Scroll')).toHaveLength(scroll?1:0);await act(async()=>tree.unmount());
});
