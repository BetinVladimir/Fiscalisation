import React from 'react';import {act,create} from 'react-test-renderer';
jest.mock('react-native',()=>({Platform:{OS:'web'},View:'Layout',ScrollView:'Scroll',StyleSheet:{create:(x:any)=>x}}));
jest.mock('react-native-paper',()=>({Surface:'Paper.Surface',Text:'Paper.Text',Appbar:{Header:'Paper.Header',Content:'Paper.Content'},useTheme:()=>({colors:{background:'#141218'}})}));
jest.mock('react-native-safe-area-context',()=>({useSafeAreaInsets:()=>({left:32,right:24,bottom:16})}));
jest.mock('../src/ui/responsive',()=>({useResponsiveLayout:()=>({gutter:16})}));
import {Screen} from '../src/ui/screen';
test('Screen uses Paper surface and scalable title',async()=>{let tree:any;await act(async()=>{tree=create(<Screen title="Long heading">Body</Screen>)});expect(tree.root.findAllByType('Paper.Surface')[0].props.elevation).toBe(0);expect(tree.root.findByType('Paper.Header').props.style.height).toBe('auto');expect(tree.root.findByType('Paper.Content').props.title.props.maxFontSizeMultiplier).not.toBe(0);await act(async()=>tree.unmount())});
test('standalone actions survive without title or scroll',async()=>{let tree:any;await act(async()=>{tree=create(<Screen scroll={false} actions={<>ACTION</>}>Body</Screen>)});expect(tree.root.findAllByType('Scroll')).toHaveLength(0);expect(tree.root.findAllByType('Paper.Header')).toHaveLength(0);expect(tree.root.findAll((node:any)=>node.children?.includes('ACTION')).length).toBeGreaterThan(0);await act(async()=>tree.unmount())});

test.each([true,false])('Paper frame retains safe area and content with scroll=%s',async scroll=>{
 let tree:any;try{await act(async()=>{tree=create(<Screen scroll={scroll} title="Heading" actions={<>ACTION</>}>Body</Screen>)});
 const surface=tree.root.findAllByType('Paper.Surface')[0];expect(surface.props.mode).toBe('flat');expect(surface.props.style).toEqual(expect.arrayContaining([expect.objectContaining({backgroundColor:'#141218'})]));
 const body=tree.root.findAllByType('Paper.Surface').find((node:any)=>Array.isArray(node.props.style)&&node.props.style.some((style:any)=>style?.paddingLeft===32));
 expect(body).toBeDefined();expect(body.props.style).toEqual(expect.arrayContaining([expect.objectContaining({paddingLeft:32,paddingRight:24,paddingBottom:16})]));
 expect(tree.root.findAllByType('Scroll')).toHaveLength(scroll?1:0);expect(tree.root.findAll((node:any)=>node.children?.includes('ACTION')).length).toBeGreaterThan(0);
 if(scroll)expect(tree.root.findByType('Scroll').props.keyboardShouldPersistTaps).toBe('handled');
 }finally{if(tree)await act(async()=>tree.unmount());}
});
