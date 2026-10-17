jest.mock('react-native-paper',()=>({Surface:'Paper.Surface'}));
import React from 'react';import {act,create} from 'react-test-renderer';
let mockFontScale=1,mockViewportWidth=320;
jest.mock('react-native',()=>({View:'Layout',useWindowDimensions:()=>({width:mockViewportWidth,fontScale:mockFontScale})}));
import {AuthForm} from '../src/ui/auth-form';
test.each([[320,1,320],[960,1,480],[1440,1,480],[960,2,960],[1440,2,480],[240,1,240]])('auth reflows within parent %i at font scale %i',async(width,scale,expected)=>{
 mockFontScale=scale;mockViewportWidth=width;let tree:any;await act(async()=>{tree=create(<AuthForm testID="form" accessibilityLabel="Sign in">Fields</AuthForm>)});
 const outer=tree.root.findAllByType('Layout')[0];await act(async()=>outer.props.onLayout({nativeEvent:{layout:{width}}}));
 const form=tree.root.findByType('Paper.Surface');expect(form.props.style[1].width).toBe(expected);expect(form.props.style[1].maxWidth).toBe('100%');expect(form.props.accessibilityLabel).toBe('Sign in');expect(JSON.stringify(tree.toJSON())).toContain('Fields');await act(async()=>tree.unmount());mockFontScale=1;
});
